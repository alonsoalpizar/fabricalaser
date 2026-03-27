-- 025_blanks_catalog.sql
-- Catálogo de blanks (productos preconfigurados): llaveros, medallas, etc.

CREATE TABLE blanks (
    id           BIGSERIAL    PRIMARY KEY,
    name         VARCHAR(100) NOT NULL,
    category     VARCHAR(50)  NOT NULL,
    description  TEXT,
    dimensions   VARCHAR(100),

    -- cost_price: costo de adquisición (solo uso interno / análisis de margen).
    -- NUNCA exponer en API pública ni al agente de WhatsApp.
    cost_price   INTEGER      NOT NULL DEFAULT 0,

    -- base_price: precio de venta unitario al cliente (a min_qty, base antes de price_breaks).
    base_price   INTEGER      NOT NULL DEFAULT 0,

    min_qty      INTEGER      NOT NULL DEFAULT 1,

    -- price_breaks: tabla de precios por volumen.
    -- Formato: [{"qty": 25, "unit_price": 240}, {"qty": 50, "unit_price": 220}]
    price_breaks JSONB        NOT NULL DEFAULT '[]',

    -- accessories: accesorios opcionales del blank.
    -- Formato: [{"name": "Argolla metálica", "price": 150, "min_qty_pack": 25}]
    accessories  JSONB        NOT NULL DEFAULT '[]',

    stock_qty    INTEGER      NOT NULL DEFAULT 0,
    stock_alert  INTEGER      NOT NULL DEFAULT 10,
    is_featured  BOOLEAN      NOT NULL DEFAULT FALSE,
    quote_count  INTEGER      NOT NULL DEFAULT 0,
    is_active    BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_blanks_category_active ON blanks (category, is_active);
CREATE INDEX idx_blanks_featured        ON blanks (is_featured DESC, quote_count DESC);

-- Seed: productos iniciales
INSERT INTO blanks (name, category, description, dimensions, cost_price, base_price, min_qty, price_breaks, accessories) VALUES
(
    'Llavero acrílico 5cm',
    'llavero',
    'Llavero de acrílico blanco o transparente, 5cm. Formas disponibles: Redondo, Cuadrado, Hexágono, Corazón, Rectángulo, Escudo. Mínimo 25 por forma. No se mezclan formas en un mismo paquete.',
    '5cm',
    120,
    240,
    25,
    '[{"qty":25,"unit_price":240},{"qty":50,"unit_price":220},{"qty":100,"unit_price":180}]',
    '[{"name":"Argolla metálica","price":150,"min_qty_pack":25}]'
),
(
    'Medalla acrílico 7cm',
    'medalla',
    'Medalla de acrílico blanco o transparente, 7cm.',
    '7cm',
    180,
    375,
    50,
    '[{"qty":50,"unit_price":375}]',
    '[]'
);
