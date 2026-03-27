-- Migration 020: Password reset tokens
-- FabricaLaser — Recuperación de contraseña

ALTER TABLE users
  ADD COLUMN IF NOT EXISTS password_reset_token  VARCHAR(64),
  ADD COLUMN IF NOT EXISTS password_reset_expires TIMESTAMPTZ;

-- Índice parcial: excluye NULLs para búsquedas rápidas de tokens activos
CREATE INDEX IF NOT EXISTS idx_users_reset_token
  ON users(password_reset_token)
  WHERE password_reset_token IS NOT NULL;
