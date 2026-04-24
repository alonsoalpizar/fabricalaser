-- Configuración del proveedor LLM activo para todos los agentes (chat web, WhatsApp/Telegram, admin chat).
-- Un solo proveedor global elegible desde /admin/configuracion-llm.html con hot reload sin reiniciar.
--
-- Claves:
--   llm_provider:  vertex|openai|deepseek|kimi|anthropic
--   llm_api_key:   AES-256-GCM encriptada con sha256(APP_SECRET) — base64 en disco
--                  Vacío para vertex (usa ADC del service account)
--   llm_model:     Modelo específico. Vacío = default por proveedor (ver internal/agent/llm/client.go)
--   llm_endpoint:  Base URL para proveedores compatibles-OpenAI. Vacío = default por proveedor
--
-- Categoría 'llm' agrupa las 4 claves para filtros futuros en el admin UI.

INSERT INTO system_config (config_key, config_value, value_type, category, description, is_active) VALUES
('llm_provider',  'vertex', 'string', 'llm', 'Proveedor LLM activo: vertex|openai|deepseek|kimi|anthropic', true),
('llm_api_key',   '',       'string', 'llm', 'API key AES-256-GCM encriptada del proveedor LLM activo (vacío = ADC para vertex)', true),
('llm_model',     '',       'string', 'llm', 'Modelo activo (vacío = default del proveedor)', true),
('llm_endpoint',  '',       'string', 'llm', 'Endpoint base para proveedores compatibles-OpenAI (vacío = default del proveedor)', true)
ON CONFLICT (config_key) DO NOTHING;
