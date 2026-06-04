-- Rollback migration 008
DROP TRIGGER IF EXISTS trg_prevent_mutation_audit_trail ON audit_trail;
DROP TABLE IF EXISTS audit_trail;
