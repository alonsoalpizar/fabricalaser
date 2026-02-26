-- Seed 002: System Configuration
-- Valores hardcodeados movidos a BD

INSERT INTO system_config (config_key, config_value, value_type, category, description) VALUES
-- Velocidades base (de time_estimator.go)
('base_engrave_area_speed', '500', 'number', 'speeds', 'Velocidad base grabado area (mm2/min)'),
('base_engrave_line_speed', '100', 'number', 'speeds', 'Velocidad base grabado linea (mm/min)'),
('base_cut_speed', '20', 'number', 'speeds', 'Velocidad base corte (mm/min)'),

-- Tiempos (de time_estimator.go)
('setup_time_minutes', '5', 'number', 'times', 'Tiempo setup por trabajo (min)'),

-- Complejidad (de calculator.go)
('complexity_auto_approve', '6.0', 'number', 'complexity', 'Factor maximo para auto-aprobacion'),
('complexity_needs_review', '12.0', 'number', 'complexity', 'Factor maximo para revision manual'),

-- Cotizaciones (de calculator.go)
('quote_validity_days', '7', 'number', 'quotes', 'Dias de validez de cotizacion'),

-- Pricing value-based (de calculator.go)
('min_value_base', '2575', 'number', 'pricing', 'Precio minimo base (CRC)'),
('price_per_mm2', '0.515', 'number', 'pricing', 'Precio por mm2 (CRC)'),
('min_area_mm2', '100', 'number', 'pricing', 'Area minima para cobrar (mm2)')
ON CONFLICT (config_key) DO UPDATE SET
    config_value = EXCLUDED.config_value,
    description = EXCLUDED.description;
