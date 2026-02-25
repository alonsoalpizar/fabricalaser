-- Migration 001: Users table
-- FabricaLaser - Sistema de cotización láser

CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    cedula VARCHAR(10) NOT NULL,
    cedula_type VARCHAR(10) NOT NULL DEFAULT 'fisica',
    nombre VARCHAR(100) NOT NULL,
    apellido VARCHAR(100),
    email VARCHAR(255) NOT NULL,
    telefono VARCHAR(20),
    password_hash VARCHAR(255),
    role VARCHAR(20) NOT NULL DEFAULT 'customer',
    quote_quota INTEGER NOT NULL DEFAULT 5,
    quotes_used INTEGER NOT NULL DEFAULT 0,
    activo BOOLEAN NOT NULL DEFAULT true,
    ultimo_login TIMESTAMP,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Unique index on cedula for users with password (registered accounts)
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_cedula_unique
ON users(cedula)
WHERE password_hash IS NOT NULL;

-- Index for login queries
CREATE INDEX IF NOT EXISTS idx_users_login
ON users(cedula, activo)
WHERE password_hash IS NOT NULL;

-- Index for email
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- Comments
COMMENT ON TABLE users IS 'Usuarios del sistema FabricaLaser';
COMMENT ON COLUMN users.cedula IS 'Cédula física (9 dígitos) o jurídica (10 dígitos)';
COMMENT ON COLUMN users.cedula_type IS 'Tipo: fisica o juridica';
COMMENT ON COLUMN users.quote_quota IS 'Cuota de cotizaciones. -1 = ilimitado';
COMMENT ON COLUMN users.quotes_used IS 'Cotizaciones consumidas';
COMMENT ON COLUMN users.metadata IS 'Datos adicionales en formato JSON';
