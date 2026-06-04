-- Migration 006: Tabela commissions
-- Ref: data-model.md §commissions; tasks.md §1.4.5; spec §FR-014 (idempotência)
-- Constitution P-I: trigger BEFORE UPDATE OR DELETE → RAISE EXCEPTION
-- Constitution P-III: value_cents BIGINT, applied_percentage NUMERIC(7,4) — zero float

CREATE TABLE commissions (
    id                 UUID           PRIMARY KEY DEFAULT gen_random_uuid(),
    -- UNIQUE em order_id garante idempotência: 1 comissão por pedido (FR-014)
    order_id           UUID           NOT NULL UNIQUE
                           REFERENCES orders (id) ON DELETE RESTRICT,
    vendor_id          UUID           NOT NULL
                           REFERENCES vendors (id) ON DELETE RESTRICT,
    value_cents        BIGINT         NOT NULL CHECK (value_cents >= 0),
    applied_percentage NUMERIC(7, 4)  NOT NULL,
    rule_id            UUID           NOT NULL
                           REFERENCES commission_rules (id) ON DELETE RESTRICT,
    period_year        INTEGER        NOT NULL,
    period_month       INTEGER        NOT NULL CHECK (period_month BETWEEN 1 AND 12),
    status             VARCHAR(20)    NOT NULL DEFAULT 'pendente'
                           CHECK (status IN ('pendente', 'aprovado', 'pago')),
    calculated_at      TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- Imutabilidade: comissão calculada não pode ser alterada ou removida (P-I)
CREATE TRIGGER trg_prevent_mutation_commissions
    BEFORE UPDATE OR DELETE ON commissions
    FOR EACH ROW EXECUTE FUNCTION fn_prevent_mutation();

-- Índice para relatório mensal por vendor
CREATE INDEX idx_commissions_vendor_period
    ON commissions (vendor_id, period_year, period_month);
