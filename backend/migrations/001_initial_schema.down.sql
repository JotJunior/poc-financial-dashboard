-- Rollback migration 001
DROP FUNCTION IF EXISTS fn_prevent_mutation();
DROP EXTENSION IF EXISTS "pgcrypto";
