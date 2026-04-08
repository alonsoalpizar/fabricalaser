-- Migration 027: Limpieza de materiales duplicados + mejoras matriz láser
-- Problemas resueltos:
--   1. Duplicados IDs 8-14 (seed corrido dos veces sin ON CONFLICT)
--   2. Vidrio ID 5 desactivado pero con speeds huérfanas
--   3. UNIQUE constraint faltante en materials
--   4. Metales puros faltantes (acero, aluminio, latón, cobre, titanio, oro, plata)
--   5. Materiales faltantes: goma/caucho, cartón
--   6. Compatibilidades incorrectas: Cerámica+Fibra, Plástico+Fibra

BEGIN;

-- =================================================================
-- 1. CLEANUP: Speeds huérfanas de Vidrio ID 5 (desactivado)
-- =================================================================

DELETE FROM tech_material_speeds WHERE material_id = 5;

-- =================================================================
-- 2. CLEANUP: Duplicados sin speeds ni quotes
--    IDs: 5, 8, 9, 10, 11, 13, 14
--    (ID 12 = Vidrio activo con quotes reales → se conserva)
-- =================================================================

DELETE FROM materials WHERE id IN (5, 8, 9, 10, 11, 13, 14);

-- =================================================================
-- 3. CONSTRAINT: Prevenir futuros duplicados de seed
-- =================================================================

ALTER TABLE materials
  ADD CONSTRAINT uq_materials_name_category UNIQUE (name, category);

-- =================================================================
-- 4. FIX: Compatibilidades incorrectas según matriz UV/Fibra
--    Cerámica → UV (Fibra técnicamente marginal, no es servicio ofrecido)
--    Plástico → UV (Fibra sin ventaja real sobre UV en plásticos)
-- =================================================================

UPDATE tech_material_speeds
SET is_compatible = false,
    notes = 'No es servicio ofrecido. UV es la tecnología correcta para cerámica.',
    updated_at = NOW()
WHERE technology_id = 3 AND material_id = 6;  -- FIBRA + Cerámica

UPDATE tech_material_speeds
SET is_compatible = false,
    notes = 'UV es la tecnología recomendada para plásticos técnicos.',
    updated_at = NOW()
WHERE technology_id = 3 AND material_id = 3;  -- FIBRA + Plástico ABS/PC

-- =================================================================
-- 5. NUEVOS MATERIALES: Metales puros → Fibra / MOPA
-- =================================================================

INSERT INTO materials (name, category, factor, thicknesses, is_cuttable, notes) VALUES
  ('Acero inoxidable', 'metal_puro', 2.00, '[]', false,
   'Marcado permanente con Fibra o MOPA. CO2 y UV no compatibles con metal puro.'),
  ('Aluminio',         'metal_puro', 1.90, '[]', false,
   'Alta reflectividad. Fibra para marcado B/N. MOPA para colores en anodizado.'),
  ('Latón',            'metal_puro', 1.85, '[]', false,
   'Aleación cobre-zinc. Fibra y MOPA. Marcado preciso y buen contraste.'),
  ('Cobre',            'metal_puro', 2.00, '[]', false,
   'Alta reflectividad. Potencia controlada. MOPA preferido sobre Fibra.'),
  ('Titanio',          'metal_puro', 2.20, '[]', false,
   'Metal duro. MOPA produce colores vivos (efecto arcoíris). Fibra para marcado B/N.'),
  ('Oro / Plata',      'metal_puro', 2.50, '[]', false,
   'Metales nobles. MOPA ideal. Requiere potencia muy controlada. Solo grabado superficial.');

-- =================================================================
-- 6. NUEVOS MATERIALES: Faltantes según matriz UV/CO2
-- =================================================================

INSERT INTO materials (name, category, factor, thicknesses, is_cuttable, notes) VALUES
  ('Goma / Caucho', 'plastico', 1.30, '[3, 5, 8, 10]', true,
   'UV recomendado (marcado frío). CO2 posible con extracción obligatoria de humos.'),
  ('Cartón',        'madera',   0.90, '[2, 3, 5]',     true,
   'CO2 ideal. Corte rápido y limpio. Factor reducido por baja densidad.');

-- =================================================================
-- 7. SPEEDS: Metales puros (Fibra y MOPA compatibles, CO2/UV no)
-- =================================================================

DO $$
DECLARE
  mat_acero    INTEGER;
  mat_aluminio INTEGER;
  mat_laton    INTEGER;
  mat_cobre    INTEGER;
  mat_titanio  INTEGER;
  mat_noble    INTEGER;
BEGIN
  SELECT id INTO mat_acero    FROM materials WHERE name = 'Acero inoxidable';
  SELECT id INTO mat_aluminio FROM materials WHERE name = 'Aluminio';
  SELECT id INTO mat_laton    FROM materials WHERE name = 'Latón';
  SELECT id INTO mat_cobre    FROM materials WHERE name = 'Cobre';
  SELECT id INTO mat_titanio  FROM materials WHERE name = 'Titanio';
  SELECT id INTO mat_noble    FROM materials WHERE name = 'Oro / Plata';

  -- Acero inoxidable
  INSERT INTO tech_material_speeds (technology_id, material_id, thickness, is_compatible, engrave_speed_mm_min, raster_speed_mm2_min, cut_speed_mm_min, notes) VALUES
    (1, mat_acero, 0, false, NULL,   NULL,  NULL, 'CO2 no marca acero directamente'),
    (2, mat_acero, 0, false, NULL,   NULL,  NULL, 'UV no compatible con metales puros'),
    (3, mat_acero, 0, true,  3000,   400,   NULL, 'Fibra: marcado permanente. Alta potencia.'),
    (4, mat_acero, 0, true,  4000,   600,   NULL, 'MOPA ideal: colores vivos en acero inox');

  -- Aluminio
  INSERT INTO tech_material_speeds (technology_id, material_id, thickness, is_compatible, engrave_speed_mm_min, raster_speed_mm2_min, cut_speed_mm_min, notes) VALUES
    (1, mat_aluminio, 0, false, NULL,  NULL,  NULL, 'CO2 no compatible con aluminio'),
    (2, mat_aluminio, 0, false, NULL,  NULL,  NULL, 'UV no compatible'),
    (3, mat_aluminio, 0, true,  4000,  700,   NULL, 'Fibra: marcado rápido y preciso'),
    (4, mat_aluminio, 0, true,  5000,  800,   NULL, 'MOPA: colores y contraste en anodizado');

  -- Latón
  INSERT INTO tech_material_speeds (technology_id, material_id, thickness, is_compatible, engrave_speed_mm_min, raster_speed_mm2_min, cut_speed_mm_min, notes) VALUES
    (1, mat_laton, 0, false, NULL,  NULL,  NULL, 'CO2 no compatible'),
    (2, mat_laton, 0, false, NULL,  NULL,  NULL, 'UV no compatible'),
    (3, mat_laton, 0, true,  3500,  500,   NULL, 'Fibra: marcado limpio y contrastado'),
    (4, mat_laton, 0, true,  4000,  600,   NULL, 'MOPA: excelente contraste en latón');

  -- Cobre
  INSERT INTO tech_material_speeds (technology_id, material_id, thickness, is_compatible, engrave_speed_mm_min, raster_speed_mm2_min, cut_speed_mm_min, notes) VALUES
    (1, mat_cobre, 0, false, NULL,  NULL,  NULL, 'CO2 no compatible'),
    (2, mat_cobre, 0, false, NULL,  NULL,  NULL, 'UV no compatible'),
    (3, mat_cobre, 0, true,  2500,  350,   NULL, 'Fibra: alta reflectividad. Potencia controlada. Prueba previa obligatoria.'),
    (4, mat_cobre, 0, true,  3000,  450,   NULL, 'MOPA preferido sobre Fibra en cobre');

  -- Titanio
  INSERT INTO tech_material_speeds (technology_id, material_id, thickness, is_compatible, engrave_speed_mm_min, raster_speed_mm2_min, cut_speed_mm_min, notes) VALUES
    (1, mat_titanio, 0, false, NULL,  NULL,  NULL, 'CO2 no compatible'),
    (2, mat_titanio, 0, false, NULL,  NULL,  NULL, 'UV no compatible'),
    (3, mat_titanio, 0, true,  2000,  300,   NULL, 'Fibra: marcado B/N preciso en titanio'),
    (4, mat_titanio, 0, true,  3000,  500,   NULL, 'MOPA: colores vivos (efecto arcoíris anodizado)');

  -- Oro / Plata
  INSERT INTO tech_material_speeds (technology_id, material_id, thickness, is_compatible, engrave_speed_mm_min, raster_speed_mm2_min, cut_speed_mm_min, notes) VALUES
    (1, mat_noble, 0, false, NULL,  NULL,  NULL, 'CO2 no compatible'),
    (2, mat_noble, 0, false, NULL,  NULL,  NULL, 'UV no compatible'),
    (3, mat_noble, 0, true,  1500,  200,   NULL, 'Fibra: potencia muy baja. Solo grabado superficial.'),
    (4, mat_noble, 0, true,  2000,  300,   NULL, 'MOPA ideal: control preciso de pulso en metales nobles');

END $$;

-- =================================================================
-- 8. SPEEDS: Goma/Caucho y Cartón
-- =================================================================

DO $$
DECLARE
  mat_goma   INTEGER;
  mat_carton INTEGER;
BEGIN
  SELECT id INTO mat_goma   FROM materials WHERE name = 'Goma / Caucho';
  SELECT id INTO mat_carton FROM materials WHERE name = 'Cartón';

  -- Goma / Caucho: UV recomendado, CO2 con precaución
  INSERT INTO tech_material_speeds (technology_id, material_id, thickness, is_compatible, engrave_speed_mm_min, raster_speed_mm2_min, cut_speed_mm_min, notes) VALUES
    (1, mat_goma, 3,  true,  3000, 700,  2000, 'CO2: posible. Extracción de humos obligatoria.'),
    (1, mat_goma, 5,  true,  3000, 700,  1500, 'CO2: revisar humos tóxicos del caucho.'),
    (2, mat_goma, 0,  true,  3500, 800,  NULL, 'UV: recomendado. Marcado frío, sin daño térmico.'),
    (3, mat_goma, 0,  false, NULL, NULL, NULL, 'Fibra no compatible con goma.'),
    (4, mat_goma, 0,  false, NULL, NULL, NULL, 'MOPA no recomendado para goma.');

  -- Cartón: CO2 ideal
  INSERT INTO tech_material_speeds (technology_id, material_id, thickness, is_compatible, engrave_speed_mm_min, raster_speed_mm2_min, cut_speed_mm_min, notes) VALUES
    (1, mat_carton, 2, true,  5000, 1200, 5000, 'CO2: corte limpio y rápido en cartón delgado.'),
    (1, mat_carton, 3, true,  5000, 1200, 4000, 'CO2: excelente resultado.'),
    (1, mat_carton, 5, true,  5000, 1200, 3000, 'CO2: 1 pasada suficiente en cartón estándar.'),
    (2, mat_carton, 0, true,  4000, 1000, NULL, 'UV: grabado fino sin quemado en bordes.'),
    (3, mat_carton, 0, false, NULL, NULL, NULL, 'Fibra no compatible con cartón.'),
    (4, mat_carton, 0, false, NULL, NULL, NULL, 'MOPA no compatible con cartón.');

END $$;

COMMIT;
