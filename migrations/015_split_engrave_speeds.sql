-- Migration: Split engrave_speed_mm_min into raster and vector speeds
-- Reason: engrave_speed_mm_min was used for both raster (mm²/min) and vector (mm/min)
--         which are different units and produce incorrect times in mixed designs.

-- Renombrar columna existente para claridad semántica
ALTER TABLE tech_material_speeds
  RENAME COLUMN engrave_speed_mm_min TO raster_speed_mm2_min;

-- Agregar columna para velocidad de grabado vectorial
ALTER TABLE tech_material_speeds
  ADD COLUMN vector_speed_mm_min DECIMAL(10,2) NULL;

-- Comentarios para documentar las unidades
COMMENT ON COLUMN tech_material_speeds.raster_speed_mm2_min IS 'Velocidad grabado raster en mm²/min (área)';
COMMENT ON COLUMN tech_material_speeds.vector_speed_mm_min IS 'Velocidad grabado vector en mm/min (líneas). NULL = usar base_engrave_line_speed';

-- Inicializar vector_speed basado en valores razonables
-- Típicamente la velocidad vector es ~10-15% de la velocidad raster para mantener proporción
-- Ejemplo: si raster = 4000 mm²/min, vector ~ 400-600 mm/min para líneas
UPDATE tech_material_speeds
SET vector_speed_mm_min = ROUND(raster_speed_mm2_min * 0.15, 2)
WHERE raster_speed_mm2_min IS NOT NULL AND raster_speed_mm2_min > 0;
