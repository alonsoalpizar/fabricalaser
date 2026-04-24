-- Migration 033: B5 — synthetic_vector_complexity_factor
-- Fecha: 2026-04-24
--
-- BuildSyntheticAnalysis antes usaba perímetro puro como VectorLengthMM,
-- subestimando cotizaciones vectoriales de WhatsApp/admin chat. El factor
-- multiplicador amplifica el perímetro para reflejar el recorrido real de
-- logos/texto (típicamente 3×-5× más que el contorno del bounding box).
-- Ajustable sin recompilar cuando se compare contra cotizaciones emitidas.

INSERT INTO system_config (config_key, config_value, value_type, category, description, is_active)
VALUES (
    'synthetic_vector_complexity_factor',
    '3.5',
    'number',
    'pricing',
    'Factor multiplicador del perímetro del bounding box para estimar VectorLengthMM en cotizaciones sin SVG (WhatsApp / admin chat). 1.0 = solo perímetro (subestima). 3.5 = estimación realista para logos/texto. Ajustar con datos reales de cotizaciones validadas.',
    TRUE
)
ON CONFLICT (config_key) DO NOTHING;
