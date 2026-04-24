-- Gestión de prompts del agente LLM desde el admin UI
--
-- Mueve los system prompts de 4 agentes Gemini (que hoy son const hardcoded en Go)
-- a DB para permitir:
--   1. Edición en tiempo real sin redeploy
--   2. Versionado completo con rollback
--   3. Auditoría de cambios (quién/cuándo/qué nota)
--   4. Hot reload via pub/sub Redis
--
-- IMPORTANTE: el body empieza vacío. El primer arranque del servicio tras esta
-- migración invoca prompts.Provider.Seed() que lo puebla con los fallbacks
-- hardcoded. Hasta ese primer arranque, los agentes sirven desde los const Go
-- (R5 del plan: len(body)==0 activa fallback).

-- Tabla principal: un registro por agente/prompt activo
CREATE TABLE agent_prompts (
    id            SERIAL PRIMARY KEY,
    agent_key     VARCHAR(50) NOT NULL UNIQUE,
    title         VARCHAR(100) NOT NULL,
    description   TEXT,
    body          TEXT NOT NULL DEFAULT '',
    version       INT NOT NULL DEFAULT 1,
    updated_by    INT REFERENCES users(id),
    is_active     BOOLEAN NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_agent_prompts_key ON agent_prompts(agent_key) WHERE is_active = true;

-- Tabla histórica: cada save crea una fila aquí con la versión anterior.
-- restored_from_version: cuando una versión se crea por rollback, apunta a la
-- versión de la que se copió el body. Permite a la UI mostrar "↩ Restaurado
-- desde v{N}" sin depender de parsear la nota. NULL = edit normal.
CREATE TABLE agent_prompt_versions (
    id                     SERIAL PRIMARY KEY,
    agent_prompt_id        INT NOT NULL REFERENCES agent_prompts(id) ON DELETE CASCADE,
    version                INT NOT NULL,
    body                   TEXT NOT NULL,
    updated_by             INT REFERENCES users(id),
    note                   TEXT,
    restored_from_version  INT,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(agent_prompt_id, version)
);

CREATE INDEX idx_agent_prompt_versions_prompt ON agent_prompt_versions(agent_prompt_id, version DESC);

-- Seed: metadata de los 5 prompts. El body queda vacío; se llena por el Seeder Go.
INSERT INTO agent_prompts (agent_key, title, description, body, version) VALUES
('chat_web_public',  'Chat Web — Público',    'Visitantes anónimos en la landing page (/chat sin auth)',  '', 1),
('chat_web_logged',  'Chat Web — Registrado', 'Usuarios autenticados en el panel (/chat con sesión)',     '', 1),
('whatsapp_main',    'WhatsApp / Telegram',   'Agente principal de mensajería instantánea',                '', 1),
('whatsapp_image',   'WhatsApp — Imágenes',   'Addon al prompt principal cuando el cliente manda imagen',  '', 1),
('admin_chat',       'Admin Chat',            'Asistente del gestor dentro del panel (/admin/asistente)',  '', 1)
ON CONFLICT (agent_key) DO NOTHING;
