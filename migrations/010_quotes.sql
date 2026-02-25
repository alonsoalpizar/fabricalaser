-- Migration 010: Quotes table for Phase 1 Cotizador
-- Stores pricing quotes calculated from SVG analyses

CREATE TABLE IF NOT EXISTS quotes (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    -- Source analysis
    svg_analysis_id INTEGER NOT NULL REFERENCES svg_analyses(id) ON DELETE CASCADE,

    -- Selected options (FK to config tables)
    technology_id INTEGER NOT NULL REFERENCES technologies(id),
    material_id INTEGER NOT NULL REFERENCES materials(id),
    engrave_type_id INTEGER NOT NULL REFERENCES engrave_types(id),

    -- Job parameters
    quantity INTEGER NOT NULL DEFAULT 1,
    thickness DECIMAL(6,2),

    -- Calculated time estimates (minutes)
    time_engrave_mins DECIMAL(10,4) DEFAULT 0,
    time_cut_mins DECIMAL(10,4) DEFAULT 0,
    time_setup_mins DECIMAL(10,4) DEFAULT 0,
    time_total_mins DECIMAL(10,4) DEFAULT 0,

    -- Pricing breakdown (from DB config - NO hardcode)
    cost_engrave DECIMAL(12,4) DEFAULT 0,
    cost_cut DECIMAL(12,4) DEFAULT 0,
    cost_setup DECIMAL(12,4) DEFAULT 0,
    cost_base DECIMAL(12,4) DEFAULT 0,
    cost_material DECIMAL(12,4) DEFAULT 0,
    cost_overhead DECIMAL(12,4) DEFAULT 0,

    -- Factors applied (from DB tables)
    factor_material DECIMAL(6,4) DEFAULT 1.0,
    factor_engrave DECIMAL(6,4) DEFAULT 1.0,
    factor_uv_premium DECIMAL(6,4) DEFAULT 0.0,
    factor_margin DECIMAL(6,4) DEFAULT 0.4,
    discount_volume_pct DECIMAL(6,4) DEFAULT 0.0,

    -- Final prices (two models)
    price_hybrid_unit DECIMAL(12,4) DEFAULT 0,
    price_hybrid_total DECIMAL(12,4) DEFAULT 0,
    price_value_unit DECIMAL(12,4) DEFAULT 0,
    price_value_total DECIMAL(12,4) DEFAULT 0,
    price_final DECIMAL(12,4) DEFAULT 0,

    -- Admin adjustments
    adjustments JSONB DEFAULT '{}',

    -- Status and workflow
    status VARCHAR(20) DEFAULT 'draft',
    review_notes TEXT,
    reviewed_by INTEGER REFERENCES users(id),
    reviewed_at TIMESTAMP WITH TIME ZONE,
    valid_until TIMESTAMP WITH TIME ZONE,
    converted_to_id INTEGER
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_quotes_user_id ON quotes(user_id);
CREATE INDEX IF NOT EXISTS idx_quotes_svg_analysis_id ON quotes(svg_analysis_id);
CREATE INDEX IF NOT EXISTS idx_quotes_status ON quotes(status);
CREATE INDEX IF NOT EXISTS idx_quotes_created_at ON quotes(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_quotes_valid_until ON quotes(valid_until);

-- Comments
COMMENT ON TABLE quotes IS 'Pricing quotes calculated from SVG analyses using DB-stored config parameters';

COMMENT ON COLUMN quotes.time_engrave_mins IS 'Calculated engrave time from area/length and engrave_type speed';
COMMENT ON COLUMN quotes.time_cut_mins IS 'Calculated cut time from cut_length and tech_rate';

COMMENT ON COLUMN quotes.factor_material IS 'From materials.factor (1.0-1.8)';
COMMENT ON COLUMN quotes.factor_engrave IS 'From engrave_types.factor (1.0-3.0)';
COMMENT ON COLUMN quotes.factor_uv_premium IS 'From technologies.uv_premium_factor';
COMMENT ON COLUMN quotes.factor_margin IS 'From tech_rates.margin_percent';
COMMENT ON COLUMN quotes.discount_volume_pct IS 'From volume_discounts based on quantity';

COMMENT ON COLUMN quotes.price_hybrid_unit IS 'Hybrid model: time × rate × factors';
COMMENT ON COLUMN quotes.price_value_unit IS 'Value model: market-based pricing';
COMMENT ON COLUMN quotes.price_final IS 'Admin-selected final price';

COMMENT ON COLUMN quotes.status IS 'draft|auto_approved|needs_review|rejected|approved|expired|converted';
COMMENT ON COLUMN quotes.adjustments IS 'JSON object for manual price adjustments';
