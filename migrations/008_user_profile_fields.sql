-- Migration 008: Add user profile fields for Costa Rica address
-- These fields allow users to complete their profile after registration

ALTER TABLE users
ADD COLUMN IF NOT EXISTS direccion VARCHAR(255),
ADD COLUMN IF NOT EXISTS provincia VARCHAR(100),
ADD COLUMN IF NOT EXISTS canton VARCHAR(100),
ADD COLUMN IF NOT EXISTS distrito VARCHAR(100);

-- Add comments for documentation
COMMENT ON COLUMN users.direccion IS 'Dirección exacta del usuario';
COMMENT ON COLUMN users.provincia IS 'Provincia de Costa Rica (7 provincias)';
COMMENT ON COLUMN users.canton IS 'Cantón dentro de la provincia';
COMMENT ON COLUMN users.distrito IS 'Distrito dentro del cantón';
