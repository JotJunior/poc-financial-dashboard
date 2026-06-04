-- Rollback migration 003
ALTER TABLE users DROP CONSTRAINT IF EXISTS fk_users_vendor_id;
DROP TABLE IF EXISTS vendors;
