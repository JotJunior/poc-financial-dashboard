-- Migration 003: Tabela vendors
-- Ref: data-model.md §vendors; tasks.md §1.4.2; checklists/compliance.md CHK077/CHK078
-- Constitution P-V LGPD: anonymized_at marca execução de anonimização

CREATE TABLE vendors (
    id             UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name           VARCHAR(200) NOT NULL,
    email          VARCHAR(255) NOT NULL UNIQUE,
    status         VARCHAR(10)  NOT NULL DEFAULT 'ativo'
                       CHECK (status IN ('ativo', 'inativo')),
    anonymized_at  TIMESTAMPTZ  NULL,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- FK de users.vendor_id → vendors.id (adicionada aqui, após vendors existir)
ALTER TABLE users
    ADD CONSTRAINT fk_users_vendor_id
    FOREIGN KEY (vendor_id) REFERENCES vendors (id)
    ON DELETE RESTRICT;

-- Índice para lookup de status ativo (listagens)
CREATE INDEX idx_vendors_status ON vendors (status);
