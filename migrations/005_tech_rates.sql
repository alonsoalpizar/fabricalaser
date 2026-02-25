-- Migration 005: Tech Rates table
-- FabricaLaser - Tarifas por tecnología

CREATE TABLE IF NOT EXISTS tech_rates (
    id SERIAL PRIMARY KEY,
    technology_id INTEGER NOT NULL REFERENCES technologies(id) ON DELETE CASCADE,
    engrave_rate_hour DECIMAL(10,4) NOT NULL,
    cut_rate_hour DECIMAL(10,4) NOT NULL,
    design_rate_hour DECIMAL(10,4) NOT NULL,
    overhead_rate_hour DECIMAL(10,4) NOT NULL DEFAULT 3.78,
    setup_fee DECIMAL(10,4) NOT NULL DEFAULT 0,
    cost_per_min_engrave DECIMAL(10,6) NOT NULL,
    cost_per_min_cut DECIMAL(10,6) NOT NULL,
    margin_percent DECIMAL(5,4) NOT NULL DEFAULT 0.40,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_tech_rates_technology ON tech_rates(technology_id);
CREATE INDEX IF NOT EXISTS idx_tech_rates_active ON tech_rates(is_active) WHERE is_active = true;

COMMENT ON TABLE tech_rates IS 'Tarifas por hora y costos por minuto por tecnología';
COMMENT ON COLUMN tech_rates.engrave_rate_hour IS 'Tarifa operador grabado USD/hora';
COMMENT ON COLUMN tech_rates.cut_rate_hour IS 'Tarifa operador corte USD/hora';
COMMENT ON COLUMN tech_rates.design_rate_hour IS 'Tarifa diseño USD/hora';
COMMENT ON COLUMN tech_rates.overhead_rate_hour IS 'Costos fijos USD/hora';
COMMENT ON COLUMN tech_rates.cost_per_min_engrave IS 'Costo total por minuto grabado';
COMMENT ON COLUMN tech_rates.cost_per_min_cut IS 'Costo total por minuto corte';
COMMENT ON COLUMN tech_rates.margin_percent IS 'Margen de ganancia (0.40 = 40%)';
