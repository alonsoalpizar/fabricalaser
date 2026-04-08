-- Migration 028: Limpieza de tech_rates duplicados + UNIQUE constraint
-- IDs 1-4: seed original en USD (engrave_rate ~10-20/hora, setup_fee=0) — INCORRECTOS
-- IDs 5-8: seed correcto en CRC (engrave_rate ~5000-10300/hora, setup_fee=4000) — CORRECTOS

BEGIN;

DELETE FROM tech_rates WHERE id IN (1, 2, 3, 4);

-- Prevenir futuros duplicados: una sola fila activa por tecnología
ALTER TABLE tech_rates
  ADD CONSTRAINT uq_tech_rates_technology UNIQUE (technology_id);

COMMIT;
