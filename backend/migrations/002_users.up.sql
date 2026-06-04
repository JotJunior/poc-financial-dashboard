-- Migration 002: Tabela users
-- Ref: data-model.md §users; tasks.md §1.4.1; checklists/security.md CHK002/CHK004
-- Constitution P-III: zero float — password_hash é TEXT (hash Argon2id, não valor monetário)

CREATE TABLE users (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name           VARCHAR(200) NOT NULL,
    email          VARCHAR(255) NOT NULL UNIQUE,
    password_hash  TEXT        NOT NULL,
    role           VARCHAR(20) NOT NULL
                       CHECK (role IN ('gestor', 'vendedor', 'financeiro')),
    vendor_id      UUID        NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Invariante: somente vendedores têm vendor_id; outros roles não têm.
    CONSTRAINT chk_vendedor_has_vendor_id
        CHECK (
            (role = 'vendedor' AND vendor_id IS NOT NULL) OR
            (role <> 'vendedor' AND vendor_id IS NULL)
        )
);

-- Índice para lookup por email (autenticação)
CREATE INDEX idx_users_email ON users (email);
