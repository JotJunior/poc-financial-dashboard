-- Migration 010: Token blocklist (JWT revogação)
-- CHK011: JWT stateless sem revogação nativa — blocklist persiste JTIs revogados.
-- Ref: tasks.md §2.2; spec §FR-024; constitution P-IV RBAC deny-by-default.

CREATE TABLE IF NOT EXISTS token_blocklist (
    jti         TEXT        NOT NULL PRIMARY KEY,       -- JWT ID (UUID v4 do campo jti)
    expires_at  TIMESTAMPTZ NOT NULL,                   -- expira conforme TTL do token original
    revoked_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),     -- quando foi revogado
    reason      VARCHAR(100)                            -- motivo: logout, user_deactivated, etc.
);

-- Índice para cleanup periódico: DELETE FROM token_blocklist WHERE expires_at < NOW()
CREATE INDEX IF NOT EXISTS idx_token_blocklist_expires_at
    ON token_blocklist (expires_at);

COMMENT ON TABLE token_blocklist IS
    'Blocklist de JTIs revogados antes da expiração natural. '
    'Limpeza periódica: DELETE WHERE expires_at < NOW(). '
    'Ref: CHK011 / tasks 2.2.';
