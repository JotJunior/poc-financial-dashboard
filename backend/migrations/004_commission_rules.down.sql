-- Rollback migration 004
DROP TRIGGER IF EXISTS trg_prevent_mutation_commission_rules ON commission_rules;
DROP TABLE IF EXISTS commission_rules;
