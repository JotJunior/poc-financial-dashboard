-- Migration 007: Tabela commission_reversals
-- Ref: data-model.md §commission_reversals; tasks.md §1.4.6; spec §FR-027/FR-028
-- Constitution P-I: trigger BEFORE UPDATE OR DELETE → RAISE EXCEPTION (imutabilidade)
-- Constitution P-III: value_cents BIGINT negativo — zero float
-- Nota: value_cents <= 0 (negativo) representa débito/estorno

CREATE TABLE commission_reversals (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    commission_id  UUID        NOT NULL
                       REFERENCES commissions (id) ON DELETE RESTRICT,
    order_id       UUID        NOT NULL
                       REFERENCES orders (id) ON DELETE RESTRICT,
    -- value_cents é negativo ou zero: representa valor estornado (débito)
    value_cents    BIGINT      NOT NULL CHECK (value_cents <= 0),
    status         VARCHAR(30) NOT NULL DEFAULT 'pendente_aprovacao'
                       CHECK (status IN ('aplicado', 'pendente_aprovacao', 'aprovado', 'lancado')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    actor_user_id  UUID        NOT NULL
                       REFERENCES users (id) ON DELETE RESTRICT
);

-- Imutabilidade: estorno não pode ser alterado ou removido (P-I)
CREATE TRIGGER trg_prevent_mutation_commission_reversals
    BEFORE UPDATE OR DELETE ON commission_reversals
    FOR EACH ROW EXECUTE FUNCTION fn_prevent_mutation();

-- Índice para listar estornos de uma comissão
CREATE INDEX idx_commission_reversals_commission_id
    ON commission_reversals (commission_id);
