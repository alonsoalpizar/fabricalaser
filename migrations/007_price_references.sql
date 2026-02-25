-- Migration 007: Price References table
-- FabricaLaser - Tabla de precios de referencia

CREATE TABLE IF NOT EXISTS price_references (
    id SERIAL PRIMARY KEY,
    service_type VARCHAR(100) NOT NULL,
    min_usd DECIMAL(10,2) NOT NULL,
    max_usd DECIMAL(10,2) NOT NULL,
    typical_time VARCHAR(50),
    description TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_price_references_type ON price_references(service_type);

COMMENT ON TABLE price_references IS 'Precios de referencia por tipo de servicio';
COMMENT ON COLUMN price_references.service_type IS 'Tipo: grabado_basico, fotograbado, corte_simple, etc.';
COMMENT ON COLUMN price_references.typical_time IS 'Tiempo típico: "1-3 min", "15-40 min"';
