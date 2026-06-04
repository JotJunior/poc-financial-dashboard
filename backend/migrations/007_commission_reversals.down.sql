-- Rollback migration 007
DROP TRIGGER IF EXISTS trg_prevent_mutation_commission_reversals ON commission_reversals;
DROP TABLE IF EXISTS commission_reversals;
