-- Rollback migration 006
DROP TRIGGER IF EXISTS trg_prevent_mutation_commissions ON commissions;
DROP TABLE IF EXISTS commissions;
