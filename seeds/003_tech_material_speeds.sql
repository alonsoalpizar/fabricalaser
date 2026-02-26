-- Seed 003: Tech Material Speeds Matrix
-- PLACEHOLDER: Velocidades de ejemplo, calibrar con pruebas reales
-- El admin puede agregar más combinaciones desde /admin/config/speeds.html

-- =====================================================
-- EJEMPLO MÍNIMO: CO2 + Madera/MDF 3mm
-- (único registro de ejemplo para validar que el sistema funciona)
-- =====================================================
INSERT INTO tech_material_speeds (technology_id, material_id, thickness, cut_speed_mm_min, engrave_speed_mm_min, is_compatible, notes)
SELECT t.id, m.id, 3.0, 30.0, 600.0, true, 'PLACEHOLDER - calibrar con pruebas reales'
FROM technologies t, materials m
WHERE t.code = 'CO2' AND m.name = 'Madera / MDF'
ON CONFLICT (technology_id, material_id, thickness) DO UPDATE SET
    cut_speed_mm_min = EXCLUDED.cut_speed_mm_min,
    engrave_speed_mm_min = EXCLUDED.engrave_speed_mm_min,
    notes = EXCLUDED.notes;

-- =====================================================
-- EJEMPLO: Material sin grosor (thickness=0)
-- Para materiales como Metal, Cuero que no tienen grosores estándar
-- =====================================================
INSERT INTO tech_material_speeds (technology_id, material_id, thickness, cut_speed_mm_min, engrave_speed_mm_min, is_compatible, notes)
SELECT t.id, m.id, 0, NULL, 600.0, true, 'PLACEHOLDER - solo grabado, sin corte'
FROM technologies t, materials m
WHERE t.code = 'FIBRA' AND m.name = 'Metal con coating'
ON CONFLICT (technology_id, material_id, thickness) DO UPDATE SET
    cut_speed_mm_min = EXCLUDED.cut_speed_mm_min,
    engrave_speed_mm_min = EXCLUDED.engrave_speed_mm_min,
    notes = EXCLUDED.notes;

-- =====================================================
-- EJEMPLO: Combinación incompatible
-- CO2 no puede trabajar con Metal
-- =====================================================
INSERT INTO tech_material_speeds (technology_id, material_id, thickness, cut_speed_mm_min, engrave_speed_mm_min, is_compatible, notes)
SELECT t.id, m.id, 0, NULL, NULL, false, 'CO2 no trabaja con metal - usar FIBRA o MOPA'
FROM technologies t, materials m
WHERE t.code = 'CO2' AND m.name = 'Metal con coating'
ON CONFLICT (technology_id, material_id, thickness) DO UPDATE SET
    is_compatible = EXCLUDED.is_compatible,
    notes = EXCLUDED.notes;
