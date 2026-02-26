-- Migration 014: Add material cost fields to quotes table
-- FabricaLaser - Fase 7: Costo de material por mm2

-- Add new columns for material cost tracking
ALTER TABLE quotes ADD COLUMN IF NOT EXISTS material_included BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE quotes ADD COLUMN IF NOT EXISTS area_consumed_mm2 DECIMAL(12,2) DEFAULT 0;
ALTER TABLE quotes ADD COLUMN IF NOT EXISTS waste_pct DECIMAL(5,4) DEFAULT 0;
ALTER TABLE quotes ADD COLUMN IF NOT EXISTS cost_material_raw DECIMAL(10,2) DEFAULT 0;
ALTER TABLE quotes ADD COLUMN IF NOT EXISTS cost_material_with_waste DECIMAL(10,2) DEFAULT 0;

COMMENT ON COLUMN quotes.material_included IS 'true si nosotros proveemos material, false si cliente provee';
COMMENT ON COLUMN quotes.area_consumed_mm2 IS 'Area del diseno en mm2 (width x height)';
COMMENT ON COLUMN quotes.waste_pct IS 'Porcentaje de merma aplicado';
COMMENT ON COLUMN quotes.cost_material_raw IS 'Costo de material antes de merma (area x cost_per_mm2)';
COMMENT ON COLUMN quotes.cost_material_with_waste IS 'Costo de material con merma (raw x (1 + waste_pct))';
