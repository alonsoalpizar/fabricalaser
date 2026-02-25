-- Migration 006: Volume Discounts table
-- FabricaLaser - Descuentos por volumen

CREATE TABLE IF NOT EXISTS volume_discounts (
    id SERIAL PRIMARY KEY,
    min_qty INTEGER NOT NULL,
    max_qty INTEGER,
    discount_pct DECIMAL(5,4) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_volume_discounts_qty ON volume_discounts(min_qty, max_qty);

COMMENT ON TABLE volume_discounts IS 'Descuentos por cantidad de piezas';
COMMENT ON COLUMN volume_discounts.min_qty IS 'Cantidad mínima para aplicar descuento';
COMMENT ON COLUMN volume_discounts.max_qty IS 'Cantidad máxima (NULL = sin límite)';
COMMENT ON COLUMN volume_discounts.discount_pct IS 'Porcentaje de descuento (0.05 = 5%)';
