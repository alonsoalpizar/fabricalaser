# ─── Variables de entorno requeridas para el webhook de WhatsApp ─────────────
# Agregar a tu archivo .env o sistema de configuración de Ubuntu

# Token de acceso a la Meta Cloud API
# Generalo en: Meta for Developers → Tu App → WhatsApp → API Setup
# IMPORTANTE: El token temporal dura ~24h. Para producción generar un System User Token permanente.
WHATSAPP_ACCESS_TOKEN=EAASapCtTiic...  # reemplazar con token completo

# ID del número de teléfono registrado en Meta
# Lo encontrás en: Meta for Developers → Tu App → WhatsApp → API Setup → Phone Number ID
WHATSAPP_PHONE_NUMBER_ID=1086125351245224

# App Secret — para verificar firma X-Hub-Signature-256 de cada webhook
# Lo encontrás en: Meta for Developers → Tu App → Settings → Basic → App Secret
WHATSAPP_APP_SECRET=

# Token que vos elegís — Meta te lo pide al configurar el webhook
# Podés usar cualquier string seguro, ej: openssl rand -hex 32
WHATSAPP_VERIFY_TOKEN=

# Versión de la API de Meta (opcional, default: v22.0)
WHATSAPP_API_VERSION=v22.0

# ─── Registro de rutas en Chi (agregar a tu router existente) ─────────────────
# En tu archivo de rutas principal (ej: internal/server/routes.go):
#
# import "tu-proyecto/internal/whatsapp"
#
# waHandler := whatsapp.NewHandler(redisClient, pgClient, geminiCaller)
#
# r.Route("/api/v1/whatsapp", func(r chi.Router) {
#     r.Get("/webhook", waHandler.VerifyWebhook)   // Handshake con Meta
#     r.Post("/webhook", waHandler.HandleMessage)  // Mensajes entrantes
# })

# ─── URL del webhook que registrás en Meta for Developers ────────────────────
# Meta for Developers → Tu App → WhatsApp → Configuration → Webhook
# Callback URL: https://fabricalaser.com/api/v1/whatsapp/webhook
# Verify Token: el mismo valor que pusiste en WHATSAPP_VERIFY_TOKEN
# Suscribirse a: messages

# ─── Tabla PostgreSQL para archivo de conversaciones ─────────────────────────
# Ejecutar en tu base de datos:
#
# CREATE TABLE whatsapp_conversations (
#     id          BIGSERIAL PRIMARY KEY,
#     phone       VARCHAR(20) NOT NULL,
#     role        VARCHAR(10) NOT NULL CHECK (role IN ('user', 'model')),
#     content     TEXT NOT NULL,
#     created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
# );
#
# CREATE INDEX idx_wa_conv_phone_created ON whatsapp_conversations (phone, created_at DESC);