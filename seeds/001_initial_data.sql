-- Seed 001: Initial Data from Simulator v5
-- FabricaLaser - Datos iniciales del simulador Excel

-- =====================================================
-- TECHNOLOGIES (4)
-- =====================================================
INSERT INTO technologies (code, name, description, uv_premium_factor, is_active) VALUES
('CO2', 'Láser CO2', 'Láser de dióxido de carbono. Ideal para madera, acrílico, cuero, tela. Potencia típica: 40-150W.', 0, true),
('UV', 'Láser UV', 'Láser ultravioleta. Ideal para vidrio, cristal, materiales delicados. Marcado frío sin daño térmico.', 0.20, true),
('FIBRA', 'Láser de Fibra', 'Láser de fibra óptica. Ideal para metales. Alta velocidad y precisión.', 0, true),
('MOPA', 'Láser MOPA', 'Láser MOPA (Master Oscillator Power Amplifier). Control de pulso avanzado para colores en metal.', 0, true)
ON CONFLICT (code) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    uv_premium_factor = EXCLUDED.uv_premium_factor;

-- =====================================================
-- MATERIALS (7) - Factores del simulador v5
-- =====================================================
INSERT INTO materials (name, category, factor, thicknesses, notes, is_active) VALUES
('Madera / MDF', 'madera', 1.0, '[3, 5, 6, 9, 12, 15, 18]', 'Material base de referencia. Factor 1.0', true),
('Acrílico transparente', 'acrilico', 1.2, '[3, 5, 6, 8, 10]', 'Calibración especial requerida. Corte limpio.', true),
('Plástico ABS/PC', 'plastico', 1.25, '[2, 3, 5]', 'Configuración especial. Cuidado con humos.', true),
('Cuero / Piel', 'cuero', 1.3, '[]', 'Material premium. Grabado suave recomendado.', true),
('Vidrio / Cristal', 'vidrio', 1.5, '[3, 4, 5, 6, 8]', 'Alto riesgo de fractura. UV ideal. Grabado superficial.', true),
('Cerámica', 'ceramica', 1.6, '[]', 'Material delicado. Requiere pruebas previas.', true),
('Metal con coating', 'metal', 1.8, '[]', 'Máxima precisión requerida. Solo grabado superficial.', true)
ON CONFLICT DO NOTHING;

-- =====================================================
-- ENGRAVE TYPES (4) - Factores del simulador v5
-- =====================================================
INSERT INTO engrave_types (name, factor, speed_multiplier, description, is_active) VALUES
('Vectorial', 1.0, 1.0, 'Grabado de líneas y contornos. Logos, texto, diagramas. El más rápido.', true),
('Rasterizado', 1.5, 0.5, 'Áreas sólidas y rellenos. Velocidad media.', true),
('Fotograbado', 2.5, 0.2, 'Imágenes con degradados y fotos. Proceso lento, alta calidad.', true),
('3D / Relieve', 3.0, 0.15, 'Múltiples pasadas para crear profundidad. El más lento.', true)
ON CONFLICT DO NOTHING;

-- =====================================================
-- TECH RATES - Tarifas UV del simulador v5
-- Engrave: $12/hr, Cut: $14/hr, Design: $15/hr, Overhead: $3.78/hr
-- Cost/min engrave = (12 + 3.78) / 60 = $0.263
-- Cost/min cut = (14 + 3.78) / 60 = $0.296
-- =====================================================
INSERT INTO tech_rates (technology_id, engrave_rate_hour, cut_rate_hour, design_rate_hour, overhead_rate_hour, setup_fee, cost_per_min_engrave, cost_per_min_cut, margin_percent, is_active)
SELECT
    t.id,
    12.00,  -- engrave_rate_hour
    14.00,  -- cut_rate_hour
    15.00,  -- design_rate_hour
    3.78,   -- overhead_rate_hour
    0.00,   -- setup_fee
    0.263,  -- cost_per_min_engrave
    0.296,  -- cost_per_min_cut
    0.40,   -- margin_percent (40%)
    true
FROM technologies t
WHERE t.code = 'UV'
ON CONFLICT DO NOTHING;

-- Rates for CO2 (similar to UV but no premium)
INSERT INTO tech_rates (technology_id, engrave_rate_hour, cut_rate_hour, design_rate_hour, overhead_rate_hour, setup_fee, cost_per_min_engrave, cost_per_min_cut, margin_percent, is_active)
SELECT
    t.id,
    10.00,  -- engrave_rate_hour (slightly lower)
    12.00,  -- cut_rate_hour
    15.00,  -- design_rate_hour
    3.78,   -- overhead_rate_hour
    0.00,   -- setup_fee
    0.230,  -- cost_per_min_engrave
    0.263,  -- cost_per_min_cut
    0.40,   -- margin_percent
    true
FROM technologies t
WHERE t.code = 'CO2'
ON CONFLICT DO NOTHING;

-- Rates for FIBRA (metal work, higher rates)
INSERT INTO tech_rates (technology_id, engrave_rate_hour, cut_rate_hour, design_rate_hour, overhead_rate_hour, setup_fee, cost_per_min_engrave, cost_per_min_cut, margin_percent, is_active)
SELECT
    t.id,
    18.00,  -- engrave_rate_hour
    20.00,  -- cut_rate_hour
    15.00,  -- design_rate_hour
    5.00,   -- overhead_rate_hour (higher for metal)
    0.00,   -- setup_fee
    0.383,  -- cost_per_min_engrave
    0.417,  -- cost_per_min_cut
    0.45,   -- margin_percent (45% for precision work)
    true
FROM technologies t
WHERE t.code = 'FIBRA'
ON CONFLICT DO NOTHING;

-- Rates for MOPA (similar to FIBRA)
INSERT INTO tech_rates (technology_id, engrave_rate_hour, cut_rate_hour, design_rate_hour, overhead_rate_hour, setup_fee, cost_per_min_engrave, cost_per_min_cut, margin_percent, is_active)
SELECT
    t.id,
    20.00,  -- engrave_rate_hour (highest for color marking)
    20.00,  -- cut_rate_hour
    15.00,  -- design_rate_hour
    5.00,   -- overhead_rate_hour
    0.00,   -- setup_fee
    0.417,  -- cost_per_min_engrave
    0.417,  -- cost_per_min_cut
    0.45,   -- margin_percent
    true
FROM technologies t
WHERE t.code = 'MOPA'
ON CONFLICT DO NOTHING;

-- =====================================================
-- VOLUME DISCOUNTS (5 ranges) - Del simulador v5
-- =====================================================
INSERT INTO volume_discounts (min_qty, max_qty, discount_pct, is_active) VALUES
(1, 9, 0.00, true),      -- 0% discount
(10, 24, 0.05, true),    -- 5% discount
(25, 49, 0.10, true),    -- 10% discount
(50, 99, 0.15, true),    -- 15% discount
(100, NULL, 0.20, true)  -- 20% discount (100+)
ON CONFLICT DO NOTHING;

-- =====================================================
-- PRICE REFERENCES (7) - Del simulador v5
-- =====================================================
INSERT INTO price_references (service_type, min_usd, max_usd, typical_time, description, is_active) VALUES
('grabado_basico', 3.00, 10.00, '1-3 min', 'Grabado básico menor a 5cm². Texto simple, logos pequeños.', true),
('grabado_estandar', 10.00, 25.00, '3-8 min', 'Grabado estándar 5-15cm². Logos medianos, texto detallado.', true),
('grabado_complejo', 25.00, 50.00, '8-15 min', 'Grabado complejo 15-30cm². Diseños elaborados.', true),
('fotograbado', 40.00, 100.00, '15-40 min', 'Fotograbado de imágenes. Requiere preparación especial.', true),
('corte_simple', 2.00, 8.00, '0.5-2 min', 'Corte simple menor a 20cm de perímetro.', true),
('corte_complejo', 8.00, 25.00, '2-8 min', 'Corte complejo mayor a 20cm. Formas intrincadas.', true),
('corte_grabado', 8.00, 40.00, '3-15 min', 'Combinación de corte y grabado en una pieza.', true)
ON CONFLICT DO NOTHING;

-- =====================================================
-- ADMIN USER
-- cedula: 999999999 (placeholder)
-- password: admin123 (bcrypt hash cost=12)
-- =====================================================
INSERT INTO users (cedula, cedula_type, nombre, email, password_hash, role, quote_quota, activo) VALUES
('999999999', 'fisica', 'Administrador', 'admin@fabricalaser.com',
 '$2a$12$2ipf2OP0eI5TH6w1eQz33.O/TuLcNiGEEyN9bzUvS0Gl79QGIInqO', -- admin123
 'admin', -1, true)
ON CONFLICT DO NOTHING;

-- =====================================================
-- Summary
-- =====================================================
-- Technologies: 4 (CO2, UV, FIBRA, MOPA)
-- Materials: 7 (with factors 1.0 - 1.8)
-- Engrave Types: 4 (with factors 1.0 - 3.0)
-- Tech Rates: 4 (one per technology)
-- Volume Discounts: 5 (0% - 20%)
-- Price References: 7
-- Admin User: 1
