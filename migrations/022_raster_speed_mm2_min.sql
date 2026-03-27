-- Agregar columna raster_speed_mm2_min a tech_material_speeds
-- Velocidad de grabado raster en mm²/min (área por minuto).
-- Cuando está poblada, el estimador la usa directamente en lugar de
-- calcular engrave_speed_mm_min × spot_size_mm (que da resultados incorrectos
-- cuando spot_size es el diámetro físico del haz y no el paso de DPI).
ALTER TABLE tech_material_speeds
    ADD COLUMN IF NOT EXISTS raster_speed_mm2_min DECIMAL(10,2) NULL;

-- Poblar todos los materiales UV con 500 mm²/min como valor inicial conservador.
-- Ajustar con mediciones reales de la máquina por material.
UPDATE tech_material_speeds
SET raster_speed_mm2_min = 500.00
WHERE technology_id = (SELECT id FROM technologies WHERE code = 'UV');
