-- Migration 035: synthetic_raster_fill_factor
-- Fecha: 2026-05-11
--
-- BuildSyntheticAnalysis usaba el bounding box completo (alto × ancho) como
-- RasterAreaMM2 para cotizaciones sin SVG (WhatsApp / admin chat), asumiendo
-- relleno del 100%. En la práctica un grabado raster ocupa típicamente 30-60%
-- del bounding box (logo con detalle, no fondo sólido), inflando los precios
-- 2-3× en piezas como medallas o placas grabadas.
--
-- Disparador: cotización Telegram 2026-05-11 200 medallas acrílicas UV →
-- ₡1,242,491 vs precio real esperado ~₡500-700k.
--
-- Este factor (0.0-1.0) se aplica como rasterAreaMM2 = alto × ancho × factor.
-- Ajustable sin recompilar para calibrar contra cotizaciones reales.

INSERT INTO system_config (config_key, config_value, value_type, category, description, is_active)
VALUES (
    'synthetic_raster_fill_factor',
    '0.5',
    'number',
    'pricing',
    'Factor de relleno (0.0-1.0) aplicado al bounding box para estimar RasterAreaMM2 en cotizaciones sin SVG (WhatsApp / admin chat). 1.0 = bounding box completo (sobreestima diseños con detalle). 0.5 = 50% del bounding box (default realista para logos/diseños con relleno parcial). Ajustar con datos reales de cotizaciones validadas.',
    TRUE
)
ON CONFLICT (config_key) DO NOTHING;
