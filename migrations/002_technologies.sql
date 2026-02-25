-- Migration 002: Technologies table
-- FabricaLaser - Tecnologías láser soportadas

CREATE TABLE IF NOT EXISTS technologies (
    id SERIAL PRIMARY KEY,
    code VARCHAR(20) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    uv_premium_factor DECIMAL(5,4) NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_technologies_code ON technologies(code);
CREATE INDEX IF NOT EXISTS idx_technologies_active ON technologies(is_active) WHERE is_active = true;

COMMENT ON TABLE technologies IS 'Tecnologías láser: CO2, UV, Fibra, MOPA';
COMMENT ON COLUMN technologies.code IS 'Código único: CO2, UV, FIBRA, MOPA';
COMMENT ON COLUMN technologies.uv_premium_factor IS 'Factor premium para UV (0.15-0.25)';
