-- Migration 029: Admin chat (asistente interno de gestores)
-- Persistencia de conversaciones del chat administrativo en /admin/asistente.html
-- Sesion = una conversacion logica del gestor (botton "nueva conversacion" la cierra)
-- Mensajes = turnos individuales (user/model/tool) con tool_calls JSONB para auditoria

BEGIN;

CREATE TABLE admin_chat_sessions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_id      INTEGER NOT NULL REFERENCES users(id),
    started_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at      TIMESTAMPTZ,
    message_count INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_admin_chat_sessions_admin_started
    ON admin_chat_sessions (admin_id, started_at DESC);

-- Indice parcial para acelerar el cierre de huerfanas (R6 del plan)
CREATE INDEX idx_admin_chat_sessions_admin_open
    ON admin_chat_sessions (admin_id) WHERE ended_at IS NULL;

CREATE TABLE admin_chat_messages (
    id          BIGSERIAL PRIMARY KEY,
    session_id  UUID NOT NULL REFERENCES admin_chat_sessions(id) ON DELETE CASCADE,
    admin_id    INTEGER NOT NULL REFERENCES users(id),
    role        VARCHAR(10) NOT NULL CHECK (role IN ('user', 'model', 'tool')),
    content     TEXT NOT NULL,
    tool_calls  JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_admin_chat_msgs_session
    ON admin_chat_messages (session_id, created_at);

CREATE INDEX idx_admin_chat_msgs_admin
    ON admin_chat_messages (admin_id, created_at DESC);

GRANT INSERT, SELECT, UPDATE ON admin_chat_sessions TO fabricalaser;
GRANT INSERT, SELECT ON admin_chat_messages TO fabricalaser;
GRANT USAGE, SELECT ON SEQUENCE admin_chat_messages_id_seq TO fabricalaser;

COMMIT;
