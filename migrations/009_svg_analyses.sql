-- Migration 009: SVG Analysis tables for Phase 1 Cotizador
-- Stores SVG analysis results and individual element geometry

-- Table: svg_analyses
-- Main analysis record with aggregated geometry
CREATE TABLE IF NOT EXISTS svg_analyses (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    filename VARCHAR(255) NOT NULL,
    file_hash VARCHAR(64),
    file_size BIGINT DEFAULT 0,
    svg_data TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    -- Dimensions (mm)
    width DECIMAL(10,4) DEFAULT 0,
    height DECIMAL(10,4) DEFAULT 0,

    -- Aggregated geometry by color classification (mm)
    cut_length_mm DECIMAL(12,4) DEFAULT 0,      -- Red stroke total
    vector_length_mm DECIMAL(12,4) DEFAULT 0,   -- Blue stroke total
    raster_area_mm2 DECIMAL(12,4) DEFAULT 0,    -- Black fill total

    -- Element counts
    element_count INTEGER DEFAULT 0,
    cut_count INTEGER DEFAULT 0,
    vector_count INTEGER DEFAULT 0,
    raster_count INTEGER DEFAULT 0,
    ignored_count INTEGER DEFAULT 0,

    -- Bounding box (mm)
    bounds_min_x DECIMAL(10,4) DEFAULT 0,
    bounds_min_y DECIMAL(10,4) DEFAULT 0,
    bounds_max_x DECIMAL(10,4) DEFAULT 0,
    bounds_max_y DECIMAL(10,4) DEFAULT 0,

    -- Status
    status VARCHAR(20) DEFAULT 'pending',
    warnings JSONB DEFAULT '[]',
    error TEXT
);

-- Table: svg_elements
-- Individual element details
CREATE TABLE IF NOT EXISTS svg_elements (
    id SERIAL PRIMARY KEY,
    analysis_id INTEGER NOT NULL REFERENCES svg_analyses(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    -- Element identification
    element_type VARCHAR(20) NOT NULL,
    element_id VARCHAR(100),

    -- Color classification
    stroke_color VARCHAR(20),
    fill_color VARCHAR(20),
    category VARCHAR(20) NOT NULL,

    -- Geometry (mm)
    length DECIMAL(12,4) DEFAULT 0,
    area DECIMAL(12,4) DEFAULT 0,
    perimeter DECIMAL(12,4) DEFAULT 0,
    points_raw TEXT,

    -- Bounding box (mm)
    bounds_min_x DECIMAL(10,4) DEFAULT 0,
    bounds_min_y DECIMAL(10,4) DEFAULT 0,
    bounds_max_x DECIMAL(10,4) DEFAULT 0,
    bounds_max_y DECIMAL(10,4) DEFAULT 0
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_svg_analyses_user_id ON svg_analyses(user_id);
CREATE INDEX IF NOT EXISTS idx_svg_analyses_file_hash ON svg_analyses(file_hash);
CREATE INDEX IF NOT EXISTS idx_svg_analyses_status ON svg_analyses(status);
CREATE INDEX IF NOT EXISTS idx_svg_analyses_created_at ON svg_analyses(created_at DESC);

CREATE INDEX IF NOT EXISTS idx_svg_elements_analysis_id ON svg_elements(analysis_id);
CREATE INDEX IF NOT EXISTS idx_svg_elements_category ON svg_elements(category);

-- Comments
COMMENT ON TABLE svg_analyses IS 'SVG file analysis results with aggregated geometry';
COMMENT ON TABLE svg_elements IS 'Individual SVG elements with color classification and geometry';

COMMENT ON COLUMN svg_analyses.cut_length_mm IS 'Total cut path length from red stroke (#FF0000) elements';
COMMENT ON COLUMN svg_analyses.vector_length_mm IS 'Total vector engrave length from blue stroke (#0000FF) elements';
COMMENT ON COLUMN svg_analyses.raster_area_mm2 IS 'Total raster engrave area from black fill (#000000) elements';
COMMENT ON COLUMN svg_analyses.status IS 'pending=uploaded, analyzed=processed, error=failed';

COMMENT ON COLUMN svg_elements.category IS 'cut=red stroke, vector=blue stroke, raster=black fill, ignored=other';
COMMENT ON COLUMN svg_elements.length IS 'Path length for cut/vector operations (mm)';
COMMENT ON COLUMN svg_elements.area IS 'Filled area for raster operations (mm2)';
