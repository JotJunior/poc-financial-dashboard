-- Migration 005: Tabelas orders e order_items
-- Ref: data-model.md §orders; tasks.md §1.4.4; spec §FR-008 (máquina de estados)
-- Constitution P-III: total_cents, unit_price_cents, line_total_cents são BIGINT — zero float

CREATE TABLE orders (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    vendor_id   UUID        NOT NULL
                    REFERENCES vendors (id) ON DELETE RESTRICT,
    total_cents BIGINT      NOT NULL CHECK (total_cents >= 0),
    order_date  DATE        NOT NULL,
    status      VARCHAR(20) NOT NULL DEFAULT 'rascunho'
                    CHECK (status IN ('rascunho', 'confirmado', 'pago', 'cancelado')),
    paid_at     TIMESTAMPTZ NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Índice para dashboard: vendas por vendor + data
CREATE INDEX idx_orders_vendor_date
    ON orders (vendor_id, order_date);

CREATE TABLE order_items (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id         UUID        NOT NULL
                         REFERENCES orders (id) ON DELETE RESTRICT,
    description      VARCHAR(500) NOT NULL,
    quantity         INTEGER     NOT NULL CHECK (quantity > 0),
    unit_price_cents BIGINT      NOT NULL CHECK (unit_price_cents >= 0),
    line_total_cents BIGINT      NOT NULL CHECK (line_total_cents >= 0),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Índice para listar itens de um pedido
CREATE INDEX idx_order_items_order_id ON order_items (order_id);
