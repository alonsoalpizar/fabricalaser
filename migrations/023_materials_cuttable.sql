-- Migration 023: is_cuttable en materials + cut_technology_id / ignore_cut_lines en quotes
-- Solo CO2 corta. Marcar explícitamente qué materiales son cortables.

ALTER TABLE materials
  ADD COLUMN IF NOT EXISTS is_cuttable BOOLEAN NOT NULL DEFAULT false;

UPDATE materials SET is_cuttable = true
WHERE name IN ('Acrílico transparente', 'Cuero / Piel', 'Madera / MDF', 'Plástico ABS/PC');

ALTER TABLE quotes
  ADD COLUMN IF NOT EXISTS cut_technology_id INTEGER REFERENCES technologies(id),
  ADD COLUMN IF NOT EXISTS ignore_cut_lines BOOLEAN NOT NULL DEFAULT false;
