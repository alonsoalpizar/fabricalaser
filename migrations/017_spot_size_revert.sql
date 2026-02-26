-- Migration: Revert split columns and add spot_size_mm to technologies
-- Reason: Instead of separate raster/vector speeds, use ONE head speed (mm/min)
--         and calculate raster speed dynamically: raster = head_speed × spot_size
--         This is simpler and more accurate (no 74 values to fill manually)

-- 1. Revert tech_material_speeds to single engrave_speed column
-- First: rename raster_speed_mm2_min back to engrave_speed_mm_min
ALTER TABLE tech_material_speeds
  RENAME COLUMN raster_speed_mm2_min TO engrave_speed_mm_min;

-- Drop the vector_speed_mm_min column (not needed with spot_size approach)
ALTER TABLE tech_material_speeds
  DROP COLUMN IF EXISTS vector_speed_mm_min;

-- Update comment to reflect new meaning
COMMENT ON COLUMN tech_material_speeds.engrave_speed_mm_min IS 'Velocidad cabezal grabado en mm/min (líneas). Para raster se multiplica por spot_size de la tecnología';

-- 2. Add spot_size_mm to technologies table
ALTER TABLE technologies
  ADD COLUMN IF NOT EXISTS spot_size_mm FLOAT NOT NULL DEFAULT 0.1;

COMMENT ON COLUMN technologies.spot_size_mm IS 'Diámetro del punto láser en mm. Usado para convertir velocidad cabezal (mm/min) a velocidad raster (mm²/min). Fórmula: raster_speed = engrave_speed × spot_size_mm';

-- 3. Set default spot sizes for existing technologies
UPDATE technologies SET spot_size_mm = 0.10 WHERE code = 'CO2';
UPDATE technologies SET spot_size_mm = 0.03 WHERE code = 'FIBRA';
UPDATE technologies SET spot_size_mm = 0.04 WHERE code = 'MOPA';
UPDATE technologies SET spot_size_mm = 0.02 WHERE code = 'UV';
