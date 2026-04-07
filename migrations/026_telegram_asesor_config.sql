INSERT INTO system_config (config_key, config_value, value_type, category, description)
VALUES ('TelegramAsesorChatID', '5814234999', 'string', 'operational', 'Chat ID de Telegram del asesor para recibir escalados')
ON CONFLICT (config_key) DO NOTHING;
