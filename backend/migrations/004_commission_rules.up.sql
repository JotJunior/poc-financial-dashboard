-- Migration 004: Tabela commission_rules
-- Ref: data-model.md §commission_rules; tasks.md §1.4.3
-- Constitution P-I: trigger BEFORE UPDATE OR DELETE → RAISE EXCEPTION (imutabilidade)
-- Constitution P-III: percentage é NUMERIC(7,4) — zero float

CREATE TABLE commission_rules (
    id          UUID           PRIMARY KEY DEFAULT gen_random_uuid(),
    vendor_id   UUID           NOT NULL
                    REFERENCES vendors (id) ON DELETE RESTRICT,
    percentage  NUMERIC(7, 4)  NOT NULL
                    CHECK (percentage >= 0 AND percentage <= 100),
    valid_from  DATE           NOT NULL,
    valid_to    DATE           NULL,
    version     INTEGER        NOT NULL,
    created_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    -- Uma regra ativa por vez: valid_to NULL ou valid_to >= valid_from
    CONSTRAINT chk_commission_rule_dates
        CHECK (valid_to IS NULL OR valid_to >= valid_from)
);

-- Imutabilidade: nenhuma regra pode ser alterada ou removida (P-I)
CREATE TRIGGER trg_prevent_mutation_commission_rules
    BEFORE UPDATE OR DELETE ON commission_rules
    FOR EACH ROW EXECUTE FUNCTION fn_prevent_mutation();

-- Índice para seleção de regra vigente por vendor e data do pedido
CREATE INDEX idx_commission_rules_vendor_dates
    ON commission_rules (vendor_id, valid_from, valid_to);
