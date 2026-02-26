-- Migration 013: Material Costs table
-- FabricaLaser - Costos de materia prima por material/grosor

CREATE TABLE IF NOT EXISTS material_costs (
    id SERIAL PRIMARY KEY,
    material_id INTEGER NOT NULL REFERENCES materials(id) ON DELETE CASCADE,
    thickness DECIMAL(5,2) NOT NULL,
    cost_per_mm2 DECIMAL(12,8) NOT NULL,
    waste_pct DECIMAL(5,4) NOT NULL DEFAULT 0.15,
    sheet_cost DECIMAL(10,2),
    sheet_width_mm DECIMAL(8,2),
    sheet_height_mm DECIMAL(8,2),
    notes TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(material_id, thickness)
);

CREATE INDEX IF NOT EXISTS idx_mc_material ON material_costs(material_id);
CREATE INDEX IF NOT EXISTS idx_mc_thickness ON material_costs(thickness);
CREATE INDEX IF NOT EXISTS idx_mc_active ON material_costs(is_active) WHERE is_active = true;

COMMENT ON TABLE material_costs IS 'Costos de materia prima por material y grosor';
COMMENT ON COLUMN material_costs.cost_per_mm2 IS 'Costo por mm2 (CRC) - calculado de sheet_cost / (width x height)';
COMMENT ON COLUMN material_costs.waste_pct IS 'Porcentaje de merma (default 15%)';
COMMENT ON COLUMN material_costs.sheet_cost IS 'Costo de lamina completa (CRC) - referencia';
COMMENT ON COLUMN material_costs.sheet_width_mm IS 'Ancho de lamina estandar (mm) - referencia';
COMMENT ON COLUMN material_costs.sheet_height_mm IS 'Alto de lamina estandar (mm) - referencia';
