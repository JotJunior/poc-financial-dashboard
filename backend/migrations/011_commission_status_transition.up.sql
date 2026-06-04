-- Migration 011: Permitir transição de status em commissions
-- Ref: spec §FR-020 (transições de comissão); tasks.md §5.1.4
-- Constitution P-I: apenas campo status é mutável; campos financeiros
--   (value_cents, applied_percentage, rule_id, etc.) permanecem imutáveis.
--
-- O trigger original fn_prevent_mutation() bloqueava QUALQUER UPDATE em commissions.
-- Para suportar aprovação/pagamento (US4), substituímos por trigger seletivo que:
--   - Permite UPDATE apenas no campo status
--   - Bloqueia qualquer mutação em campos financeiros (P-II, P-III)

-- Remover trigger geral de imutabilidade da tabela commissions
DROP TRIGGER IF EXISTS trg_prevent_mutation_commissions ON commissions;

-- Nova função seletiva: permite apenas status, bloqueia campos financeiros
CREATE OR REPLACE FUNCTION fn_commissions_status_only()
RETURNS TRIGGER AS $$
BEGIN
    -- Bloquear DELETE sempre
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'DELETE proibido em commissions (constitution P-I — auditabilidade)';
    END IF;

    -- Para UPDATE: verificar que nenhum campo financeiro foi alterado
    IF NEW.order_id       IS DISTINCT FROM OLD.order_id       THEN
        RAISE EXCEPTION 'Mutação de order_id proibida em commissions (P-I)';
    END IF;
    IF NEW.vendor_id      IS DISTINCT FROM OLD.vendor_id      THEN
        RAISE EXCEPTION 'Mutação de vendor_id proibida em commissions (P-I)';
    END IF;
    IF NEW.value_cents    IS DISTINCT FROM OLD.value_cents    THEN
        RAISE EXCEPTION 'Mutação de value_cents proibida em commissions (P-II/P-III)';
    END IF;
    IF NEW.applied_percentage IS DISTINCT FROM OLD.applied_percentage THEN
        RAISE EXCEPTION 'Mutação de applied_percentage proibida em commissions (P-II/P-III)';
    END IF;
    IF NEW.rule_id        IS DISTINCT FROM OLD.rule_id        THEN
        RAISE EXCEPTION 'Mutação de rule_id proibida em commissions (P-I)';
    END IF;
    IF NEW.period_year    IS DISTINCT FROM OLD.period_year    THEN
        RAISE EXCEPTION 'Mutação de period_year proibida em commissions (P-I)';
    END IF;
    IF NEW.period_month   IS DISTINCT FROM OLD.period_month   THEN
        RAISE EXCEPTION 'Mutação de period_month proibida em commissions (P-I)';
    END IF;
    IF NEW.calculated_at  IS DISTINCT FROM OLD.calculated_at  THEN
        RAISE EXCEPTION 'Mutação de calculated_at proibida em commissions (P-I)';
    END IF;

    -- Apenas status pode mudar — RETURN NEW para permitir
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Registrar novo trigger seletivo
CREATE TRIGGER trg_commissions_status_only
    BEFORE UPDATE OR DELETE ON commissions
    FOR EACH ROW EXECUTE FUNCTION fn_commissions_status_only();
