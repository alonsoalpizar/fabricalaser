-- migrations/018_tech_rates_overhead_columns.sql
-- Agregar columnas de costos fijos por máquina a tech_rates
-- Estos costos son específicos de cada tecnología/máquina

ALTER TABLE tech_rates
  ADD COLUMN IF NOT EXISTS electricidad_mes FLOAT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS mantenimiento_mes FLOAT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS depreciacion_mes FLOAT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS seguro_mes FLOAT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS consumibles_mes FLOAT NOT NULL DEFAULT 0;

COMMENT ON COLUMN tech_rates.electricidad_mes IS 'Electricidad mensual de esta máquina (₡)';
COMMENT ON COLUMN tech_rates.mantenimiento_mes IS 'Mantenimiento preventivo mensual de esta máquina (₡)';
COMMENT ON COLUMN tech_rates.depreciacion_mes IS 'Depreciación mensual del equipo (₡) - costo/meses_vida_util';
COMMENT ON COLUMN tech_rates.seguro_mes IS 'Seguro mensual del equipo (₡)';
COMMENT ON COLUMN tech_rates.consumibles_mes IS 'Consumibles mensuales de esta máquina (₡)';

-- Valores iniciales del Simulador Excel v5 — Alonso ajustará por tecnología real
UPDATE tech_rates SET
    electricidad_mes = 15450,
    mantenimiento_mes = 25750,
    depreciacion_mes = 96990,
    seguro_mes = 12875,
    consumibles_mes = 20600
WHERE technology_id IN (SELECT id FROM technologies);
