-- Migration 030: Aliases por blank (sinónimos de búsqueda semántica)
-- Permite al agente Gemini del chat admin matchear expresiones como
-- "acrílicos redondos" con el blank "llavero acrílico 5cm" sin hardcodear
-- los sinónimos en el código Go (internal/handlers/admin/chat/context_provider.go).
--
-- Formato del campo: array de strings JSON. Ejemplo:
--   ["acrílicos redondos", "discos de acrílico", "llavero para personalizar"]

BEGIN;

ALTER TABLE blanks
    ADD COLUMN IF NOT EXISTS aliases JSONB NOT NULL DEFAULT '[]'::jsonb;

-- Backfill con los aliases que hasta ahora vivían hardcoded en
-- context_provider.go:categoryAliases(). Solo tocamos las categorías
-- que tenían aliases definidos. Futuros blanks nacen con '[]'.

UPDATE blanks
SET aliases = '["acrílicos redondos","discos de acrílico","piezas para sublimar","llaveros para personalizar","llavero blanco","llavero transparente","círculos acrílico"]'::jsonb
WHERE category = 'llavero' AND aliases = '[]'::jsonb;

UPDATE blanks
SET aliases = '["medallas para grabar","medallones acrílico","premios acrílico","piezas redondas 7cm con ranura"]'::jsonb
WHERE category = 'medalla' AND aliases = '[]'::jsonb;

COMMIT;
