-- Migration 011: System Configuration table
-- FabricaLaser - Configuracion general del sistema

CREATE TABLE IF NOT EXISTS system_config (
    id SERIAL PRIMARY KEY,
    config_key VARCHAR(100) UNIQUE NOT NULL,
    config_value TEXT NOT NULL,
    value_type VARCHAR(20) NOT NULL DEFAULT 'string',
    category VARCHAR(50) NOT NULL,
    description TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_system_config_key ON system_config(config_key);
CREATE INDEX IF NOT EXISTS idx_system_config_category ON system_config(category);
CREATE INDEX IF NOT EXISTS idx_system_config_active ON system_config(is_active) WHERE is_active = true;

COMMENT ON TABLE system_config IS 'Configuracion general del sistema (valores hardcodeados movidos a BD)';
COMMENT ON COLUMN system_config.config_key IS 'Clave unica de configuracion';
COMMENT ON COLUMN system_config.config_value IS 'Valor como texto (parsear segun value_type)';
COMMENT ON COLUMN system_config.value_type IS 'Tipo: string, number, boolean, json';
COMMENT ON COLUMN system_config.category IS 'Categoria: speeds, times, complexity, pricing, quotes';
