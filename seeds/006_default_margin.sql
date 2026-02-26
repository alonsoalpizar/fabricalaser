-- Default margin percent for technologies without specific tech_rate
INSERT INTO system_config (config_key, config_value, value_type, category, description, is_active)
VALUES ('default_margin_percent', '0.40', 'number', 'pricing', 'Margen por defecto si no hay tech_rate configurado', true)
ON CONFLICT (config_key) DO NOTHING;
