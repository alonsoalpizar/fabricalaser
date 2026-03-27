-- Migración 024: Agregar material Corcho al catálogo
-- Corcho: orgánico poroso, excelente con CO2, factor 1.1 por variabilidad en absorción
-- Velocidades estimadas con base en Madera/MDF (factor ~5x menor densidad favorece corte)

BEGIN;

-- 1. Insertar material
INSERT INTO materials (name, category, factor, is_active, is_cuttable, notes)
VALUES (
  'Corcho',
  'madera',
  1.10,
  true,
  true,
  'Material poroso y orgánico. Excelente respuesta con CO2. Requiere extracción de polvo y humos. Prueba previa recomendada en espesores >5mm.'
);

-- 2. Capturar el ID asignado
DO $$
DECLARE
  mat_id INTEGER;
BEGIN
  SELECT id INTO mat_id FROM materials WHERE name = 'Corcho' AND is_active = true;

  -- CO2 (id=1) — tecnología principal para corcho: graba y corta bien
  INSERT INTO tech_material_speeds
    (technology_id, material_id, thickness, is_compatible, engrave_speed_mm_min, raster_speed_mm2_min, cut_speed_mm_min, notes)
  VALUES
    (1, mat_id, 3.00,  true, 4000.00, 900.00, 3500.00, 'Corte rápido por baja densidad. Verificar extracción de polvo.'),
    (1, mat_id, 5.00,  true, 4000.00, 900.00, 2500.00, 'Buen resultado. Revisar bordes si el corcho tiene irregularidades.'),
    (1, mat_id, 10.00, true, 4000.00, 900.00, 1200.00, 'Espesores altos: considerar 2 pasadas a 600 mm/min c/u.');

  -- UV (id=2) — compatible para grabado fino, sin capacidad de corte
  INSERT INTO tech_material_speeds
    (technology_id, material_id, thickness, is_compatible, engrave_speed_mm_min, raster_speed_mm2_min, cut_speed_mm_min, notes)
  VALUES
    (2, mat_id, 0.00, true, 2000.00, 500.00, NULL, 'Grabado fino. UV no corta corcho efectivamente.');

  -- FIBRA (id=3) — no compatible
  INSERT INTO tech_material_speeds
    (technology_id, material_id, thickness, is_compatible, engrave_speed_mm_min, raster_speed_mm2_min, cut_speed_mm_min, notes)
  VALUES
    (3, mat_id, 0.00, false, NULL, NULL, NULL, 'No compatible. Riesgo de quemado irregular y fuego por estructura porosa.');

  -- MOPA (id=4) — no compatible
  INSERT INTO tech_material_speeds
    (technology_id, material_id, thickness, is_compatible, engrave_speed_mm_min, raster_speed_mm2_min, cut_speed_mm_min, notes)
  VALUES
    (4, mat_id, 0.00, false, NULL, NULL, NULL, 'No recomendado para corcho.');

END $$;

COMMIT;
