-- Migration 034: B4 — minimum_order_amount + audit columns on quotes
-- Fecha: 2026-04-24
--
-- Pedidos muy pequeños pueden producir precios finales que no cubren el costo
-- real de setup + tiempo mínimo de máquina. Este piso lleva el precio ganador
-- (Hybrid o Value, el que ganó el MAX) hasta un mínimo configurable en CRC.
-- El modelo no ganador queda como referencia informativa.
--
-- 0 en config desactiva la regla.

INSERT INTO system_config (config_key, config_value, value_type, category, description, is_active)
VALUES (
    'minimum_order_amount',
    '5000',
    'number',
    'pricing',
    'Precio mínimo final de una orden en CRC. Si el precio ganador (Hybrid o Value) queda por debajo, se levanta al mínimo. 0 desactiva la regla.',
    TRUE
)
ON CONFLICT (config_key) DO NOTHING;

ALTER TABLE quotes
    ADD COLUMN IF NOT EXISTS min_order_applied BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS min_order_amount DECIMAL(12, 2) NOT NULL DEFAULT 0;
