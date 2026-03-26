-- Tabla para archivar conversaciones de WhatsApp
CREATE TABLE whatsapp_conversations (
    id         BIGSERIAL PRIMARY KEY,
    phone      VARCHAR(20) NOT NULL,
    role       VARCHAR(10) NOT NULL CHECK (role IN ('user', 'model')),
    content    TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_wa_conv_phone_created ON whatsapp_conversations (phone, created_at DESC);
