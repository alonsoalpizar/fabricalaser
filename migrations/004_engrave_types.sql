-- Migration 004: Engrave Types table
-- FabricaLaser - Tipos de grabado con factores de tiempo

CREATE TABLE IF NOT EXISTS engrave_types (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) NOT NULL,
    factor DECIMAL(5,4) NOT NULL DEFAULT 1.0,
    speed_multiplier DECIMAL(5,4) NOT NULL DEFAULT 1.0,
    description TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_engrave_types_active ON engrave_types(is_active) WHERE is_active = true;

COMMENT ON TABLE engrave_types IS 'Tipos de grabado: vectorial, rasterizado, fotograbado, 3D';
COMMENT ON COLUMN engrave_types.factor IS 'Factor de tiempo (1.0 - 3.0)';
COMMENT ON COLUMN engrave_types.speed_multiplier IS 'Multiplicador de velocidad relativa';
