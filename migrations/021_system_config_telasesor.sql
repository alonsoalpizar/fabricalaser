-- Insertar teléfono del asesor de ventas para escalado desde WhatsApp
INSERT INTO system_config (config_key, config_value, value_type, category, description, is_active)
VALUES ('TelAsesor', '+50686091954', 'string', 'operational',
        'Teléfono del asesor de ventas para escalado desde WhatsApp (formato E.164)', true)
ON CONFLICT (config_key) DO NOTHING;
