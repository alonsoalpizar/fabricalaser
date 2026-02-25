-- Migration 003: Materials table
-- FabricaLaser - Materiales con factores de ajuste

CREATE TABLE IF NOT EXISTS materials (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    category VARCHAR(50) NOT NULL,
    factor DECIMAL(5,4) NOT NULL DEFAULT 1.0,
    thicknesses JSONB NOT NULL DEFAULT '[]',
    notes TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_materials_category ON materials(category);
CREATE INDEX IF NOT EXISTS idx_materials_active ON materials(is_active) WHERE is_active = true;

COMMENT ON TABLE materials IS 'Materiales para corte/grabado con factores de ajuste';
COMMENT ON COLUMN materials.factor IS 'Factor de ajuste de precio (1.0 - 1.8)';
COMMENT ON COLUMN materials.thicknesses IS 'Espesores disponibles en mm: [3, 5, 6, 10]';
