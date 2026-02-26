-- New fields for quote model from FASE 2 and 3
ALTER TABLE quotes ADD COLUMN IF NOT EXISTS price_model VARCHAR(10) DEFAULT 'hybrid';
ALTER TABLE quotes ADD COLUMN IF NOT EXISTS sim_hybrid_with_material_factor DECIMAL(12,2) DEFAULT 0;
ALTER TABLE quotes ADD COLUMN IF NOT EXISTS sim_difference_pct DECIMAL(8,4) DEFAULT 0;
ALTER TABLE quotes ADD COLUMN IF NOT EXISTS used_fallback_speeds BOOLEAN DEFAULT false;
ALTER TABLE quotes ADD COLUMN IF NOT EXISTS fallback_warning TEXT;
