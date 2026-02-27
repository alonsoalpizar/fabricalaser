-- seeds/008_overhead_costs.sql
-- Costos fijos GLOBALES del taller (₡/mes) — compartidos entre todas las tecnologías
-- Los costos por máquina (electricidad, mantenimiento, etc.) están en tech_rates

INSERT INTO system_config (config_key, config_value, value_type, category, description) VALUES
('overhead_alquiler', '51500', 'number', 'overhead_global', 'Alquiler espacio mensual (₡) - costo oportunidad'),
('overhead_internet', '10300', 'number', 'overhead_global', 'Internet/servicios proporcional mensual (₡)'),
('horas_trabajo_mes', '120', 'number', 'overhead_global', 'Horas de trabajo estimadas por mes')
ON CONFLICT (config_key) DO UPDATE SET
    config_value = EXCLUDED.config_value,
    category = EXCLUDED.category,
    description = EXCLUDED.description;
