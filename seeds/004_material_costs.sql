-- Seed 004: Material Costs
-- Costos de materia prima (datos reales aproximados)
-- Lamina estandar: 1220 x 2440 mm = 2,976,800 mm2

-- =====================================================
-- Acrilico transparente
-- =====================================================
INSERT INTO material_costs (material_id, thickness, cost_per_mm2, waste_pct, sheet_cost, sheet_width_mm, sheet_height_mm, notes)
SELECT m.id, 3.0, 0.00503935, 0.15, 15000.00, 1220.00, 2440.00, 'Acrilico 3mm - lamina estandar'
FROM materials m WHERE m.name = 'Acrílico transparente'
ON CONFLICT (material_id, thickness) DO UPDATE SET
    cost_per_mm2 = EXCLUDED.cost_per_mm2,
    sheet_cost = EXCLUDED.sheet_cost,
    notes = EXCLUDED.notes;

INSERT INTO material_costs (material_id, thickness, cost_per_mm2, waste_pct, sheet_cost, sheet_width_mm, sheet_height_mm, notes)
SELECT m.id, 5.0, 0.00739102, 0.15, 22000.00, 1220.00, 2440.00, 'Acrilico 5mm - lamina estandar'
FROM materials m WHERE m.name = 'Acrílico transparente'
ON CONFLICT (material_id, thickness) DO UPDATE SET
    cost_per_mm2 = EXCLUDED.cost_per_mm2,
    sheet_cost = EXCLUDED.sheet_cost,
    notes = EXCLUDED.notes;

INSERT INTO material_costs (material_id, thickness, cost_per_mm2, waste_pct, sheet_cost, sheet_width_mm, sheet_height_mm, notes)
SELECT m.id, 6.0, 0.00873689, 0.15, 26000.00, 1220.00, 2440.00, 'Acrilico 6mm - lamina estandar'
FROM materials m WHERE m.name = 'Acrílico transparente'
ON CONFLICT (material_id, thickness) DO UPDATE SET
    cost_per_mm2 = EXCLUDED.cost_per_mm2,
    sheet_cost = EXCLUDED.sheet_cost,
    notes = EXCLUDED.notes;

INSERT INTO material_costs (material_id, thickness, cost_per_mm2, waste_pct, sheet_cost, sheet_width_mm, sheet_height_mm, notes)
SELECT m.id, 8.0, 0.01074649, 0.15, 32000.00, 1220.00, 2440.00, 'Acrilico 8mm - lamina estandar'
FROM materials m WHERE m.name = 'Acrílico transparente'
ON CONFLICT (material_id, thickness) DO UPDATE SET
    cost_per_mm2 = EXCLUDED.cost_per_mm2,
    sheet_cost = EXCLUDED.sheet_cost,
    notes = EXCLUDED.notes;

INSERT INTO material_costs (material_id, thickness, cost_per_mm2, waste_pct, sheet_cost, sheet_width_mm, sheet_height_mm, notes)
SELECT m.id, 10.0, 0.01343822, 0.15, 40000.00, 1220.00, 2440.00, 'Acrilico 10mm - lamina estandar'
FROM materials m WHERE m.name = 'Acrílico transparente'
ON CONFLICT (material_id, thickness) DO UPDATE SET
    cost_per_mm2 = EXCLUDED.cost_per_mm2,
    sheet_cost = EXCLUDED.sheet_cost,
    notes = EXCLUDED.notes;

-- =====================================================
-- Madera / MDF
-- =====================================================
INSERT INTO material_costs (material_id, thickness, cost_per_mm2, waste_pct, sheet_cost, sheet_width_mm, sheet_height_mm, notes)
SELECT m.id, 3.0, 0.00167978, 0.15, 5000.00, 1220.00, 2440.00, 'MDF 3mm - lamina estandar'
FROM materials m WHERE m.name = 'Madera / MDF'
ON CONFLICT (material_id, thickness) DO UPDATE SET
    cost_per_mm2 = EXCLUDED.cost_per_mm2,
    sheet_cost = EXCLUDED.sheet_cost,
    notes = EXCLUDED.notes;

INSERT INTO material_costs (material_id, thickness, cost_per_mm2, waste_pct, sheet_cost, sheet_width_mm, sheet_height_mm, notes)
SELECT m.id, 5.0, 0.00251967, 0.15, 7500.00, 1220.00, 2440.00, 'MDF 5mm - lamina estandar'
FROM materials m WHERE m.name = 'Madera / MDF'
ON CONFLICT (material_id, thickness) DO UPDATE SET
    cost_per_mm2 = EXCLUDED.cost_per_mm2,
    sheet_cost = EXCLUDED.sheet_cost,
    notes = EXCLUDED.notes;

INSERT INTO material_costs (material_id, thickness, cost_per_mm2, waste_pct, sheet_cost, sheet_width_mm, sheet_height_mm, notes)
SELECT m.id, 6.0, 0.00285542, 0.15, 8500.00, 1220.00, 2440.00, 'MDF 6mm - lamina estandar'
FROM materials m WHERE m.name = 'Madera / MDF'
ON CONFLICT (material_id, thickness) DO UPDATE SET
    cost_per_mm2 = EXCLUDED.cost_per_mm2,
    sheet_cost = EXCLUDED.sheet_cost,
    notes = EXCLUDED.notes;

INSERT INTO material_costs (material_id, thickness, cost_per_mm2, waste_pct, sheet_cost, sheet_width_mm, sheet_height_mm, notes)
SELECT m.id, 9.0, 0.00335956, 0.15, 10000.00, 1220.00, 2440.00, 'MDF 9mm - lamina estandar'
FROM materials m WHERE m.name = 'Madera / MDF'
ON CONFLICT (material_id, thickness) DO UPDATE SET
    cost_per_mm2 = EXCLUDED.cost_per_mm2,
    sheet_cost = EXCLUDED.sheet_cost,
    notes = EXCLUDED.notes;

INSERT INTO material_costs (material_id, thickness, cost_per_mm2, waste_pct, sheet_cost, sheet_width_mm, sheet_height_mm, notes)
SELECT m.id, 12.0, 0.00403147, 0.15, 12000.00, 1220.00, 2440.00, 'MDF 12mm - lamina estandar'
FROM materials m WHERE m.name = 'Madera / MDF'
ON CONFLICT (material_id, thickness) DO UPDATE SET
    cost_per_mm2 = EXCLUDED.cost_per_mm2,
    sheet_cost = EXCLUDED.sheet_cost,
    notes = EXCLUDED.notes;

INSERT INTO material_costs (material_id, thickness, cost_per_mm2, waste_pct, sheet_cost, sheet_width_mm, sheet_height_mm, notes)
SELECT m.id, 15.0, 0.00503935, 0.15, 15000.00, 1220.00, 2440.00, 'MDF 15mm - lamina estandar'
FROM materials m WHERE m.name = 'Madera / MDF'
ON CONFLICT (material_id, thickness) DO UPDATE SET
    cost_per_mm2 = EXCLUDED.cost_per_mm2,
    sheet_cost = EXCLUDED.sheet_cost,
    notes = EXCLUDED.notes;

INSERT INTO material_costs (material_id, thickness, cost_per_mm2, waste_pct, sheet_cost, sheet_width_mm, sheet_height_mm, notes)
SELECT m.id, 18.0, 0.00604722, 0.15, 18000.00, 1220.00, 2440.00, 'MDF 18mm - lamina estandar'
FROM materials m WHERE m.name = 'Madera / MDF'
ON CONFLICT (material_id, thickness) DO UPDATE SET
    cost_per_mm2 = EXCLUDED.cost_per_mm2,
    sheet_cost = EXCLUDED.sheet_cost,
    notes = EXCLUDED.notes;

-- =====================================================
-- Plastico ABS/PC
-- =====================================================
INSERT INTO material_costs (material_id, thickness, cost_per_mm2, waste_pct, sheet_cost, sheet_width_mm, sheet_height_mm, notes)
SELECT m.id, 2.0, 0.00604722, 0.15, 18000.00, 1220.00, 2440.00, 'ABS 2mm - lamina estandar'
FROM materials m WHERE m.name = 'Plástico ABS/PC'
ON CONFLICT (material_id, thickness) DO UPDATE SET
    cost_per_mm2 = EXCLUDED.cost_per_mm2,
    sheet_cost = EXCLUDED.sheet_cost,
    notes = EXCLUDED.notes;

INSERT INTO material_costs (material_id, thickness, cost_per_mm2, waste_pct, sheet_cost, sheet_width_mm, sheet_height_mm, notes)
SELECT m.id, 3.0, 0.00739102, 0.15, 22000.00, 1220.00, 2440.00, 'ABS 3mm - lamina estandar'
FROM materials m WHERE m.name = 'Plástico ABS/PC'
ON CONFLICT (material_id, thickness) DO UPDATE SET
    cost_per_mm2 = EXCLUDED.cost_per_mm2,
    sheet_cost = EXCLUDED.sheet_cost,
    notes = EXCLUDED.notes;

INSERT INTO material_costs (material_id, thickness, cost_per_mm2, waste_pct, sheet_cost, sheet_width_mm, sheet_height_mm, notes)
SELECT m.id, 5.0, 0.01007869, 0.15, 30000.00, 1220.00, 2440.00, 'ABS 5mm - lamina estandar'
FROM materials m WHERE m.name = 'Plástico ABS/PC'
ON CONFLICT (material_id, thickness) DO UPDATE SET
    cost_per_mm2 = EXCLUDED.cost_per_mm2,
    sheet_cost = EXCLUDED.sheet_cost,
    notes = EXCLUDED.notes;

-- =====================================================
-- Vidrio / Cristal
-- =====================================================
INSERT INTO material_costs (material_id, thickness, cost_per_mm2, waste_pct, sheet_cost, sheet_width_mm, sheet_height_mm, notes)
SELECT m.id, 3.0, 0.00403147, 0.20, 12000.00, 1220.00, 2440.00, 'Vidrio 3mm - mayor merma por fragilidad'
FROM materials m WHERE m.name = 'Vidrio / Cristal'
ON CONFLICT (material_id, thickness) DO UPDATE SET
    cost_per_mm2 = EXCLUDED.cost_per_mm2,
    waste_pct = EXCLUDED.waste_pct,
    sheet_cost = EXCLUDED.sheet_cost,
    notes = EXCLUDED.notes;

INSERT INTO material_costs (material_id, thickness, cost_per_mm2, waste_pct, sheet_cost, sheet_width_mm, sheet_height_mm, notes)
SELECT m.id, 4.0, 0.00503935, 0.20, 15000.00, 1220.00, 2440.00, 'Vidrio 4mm - mayor merma por fragilidad'
FROM materials m WHERE m.name = 'Vidrio / Cristal'
ON CONFLICT (material_id, thickness) DO UPDATE SET
    cost_per_mm2 = EXCLUDED.cost_per_mm2,
    waste_pct = EXCLUDED.waste_pct,
    sheet_cost = EXCLUDED.sheet_cost,
    notes = EXCLUDED.notes;

INSERT INTO material_costs (material_id, thickness, cost_per_mm2, waste_pct, sheet_cost, sheet_width_mm, sheet_height_mm, notes)
SELECT m.id, 5.0, 0.00604722, 0.20, 18000.00, 1220.00, 2440.00, 'Vidrio 5mm - mayor merma por fragilidad'
FROM materials m WHERE m.name = 'Vidrio / Cristal'
ON CONFLICT (material_id, thickness) DO UPDATE SET
    cost_per_mm2 = EXCLUDED.cost_per_mm2,
    waste_pct = EXCLUDED.waste_pct,
    sheet_cost = EXCLUDED.sheet_cost,
    notes = EXCLUDED.notes;

INSERT INTO material_costs (material_id, thickness, cost_per_mm2, waste_pct, sheet_cost, sheet_width_mm, sheet_height_mm, notes)
SELECT m.id, 6.0, 0.00705509, 0.20, 21000.00, 1220.00, 2440.00, 'Vidrio 6mm - mayor merma por fragilidad'
FROM materials m WHERE m.name = 'Vidrio / Cristal'
ON CONFLICT (material_id, thickness) DO UPDATE SET
    cost_per_mm2 = EXCLUDED.cost_per_mm2,
    waste_pct = EXCLUDED.waste_pct,
    sheet_cost = EXCLUDED.sheet_cost,
    notes = EXCLUDED.notes;

INSERT INTO material_costs (material_id, thickness, cost_per_mm2, waste_pct, sheet_cost, sheet_width_mm, sheet_height_mm, notes)
SELECT m.id, 8.0, 0.00873689, 0.20, 26000.00, 1220.00, 2440.00, 'Vidrio 8mm - mayor merma por fragilidad'
FROM materials m WHERE m.name = 'Vidrio / Cristal'
ON CONFLICT (material_id, thickness) DO UPDATE SET
    cost_per_mm2 = EXCLUDED.cost_per_mm2,
    waste_pct = EXCLUDED.waste_pct,
    sheet_cost = EXCLUDED.sheet_cost,
    notes = EXCLUDED.notes;

-- =====================================================
-- Materiales sin grosor (cliente provee material)
-- Metal, Cuero, Ceramica: thickness=0, cost=0
-- =====================================================
INSERT INTO material_costs (material_id, thickness, cost_per_mm2, waste_pct, notes)
SELECT m.id, 0, 0, 0, 'Cliente provee material'
FROM materials m WHERE m.name = 'Metal con coating'
ON CONFLICT (material_id, thickness) DO NOTHING;

INSERT INTO material_costs (material_id, thickness, cost_per_mm2, waste_pct, notes)
SELECT m.id, 0, 0, 0, 'Cliente provee material'
FROM materials m WHERE m.name = 'Cuero / Piel'
ON CONFLICT (material_id, thickness) DO NOTHING;

INSERT INTO material_costs (material_id, thickness, cost_per_mm2, waste_pct, notes)
SELECT m.id, 0, 0, 0, 'Cliente provee material'
FROM materials m WHERE m.name = 'Cerámica'
ON CONFLICT (material_id, thickness) DO NOTHING;
