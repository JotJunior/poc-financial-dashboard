-- Rollback de 011: restaurar trigger geral de imutabilidade
DROP TRIGGER IF EXISTS trg_commissions_status_only ON commissions;
DROP FUNCTION IF EXISTS fn_commissions_status_only();

CREATE TRIGGER trg_prevent_mutation_commissions
    BEFORE UPDATE OR DELETE ON commissions
    FOR EACH ROW EXECUTE FUNCTION fn_prevent_mutation();
