-- Migration 008: Tabela audit_trail
-- Ref: data-model.md §audit_trail; tasks.md §1.4.7; checklists/compliance.md CHK069/CHK078
-- Constitution P-I: toda transição de estado é auditável; tabela append-only via trigger
-- Sem colunas monetárias — metadata é JSONB (estrutura arbitrária, não valor monetário)

CREATE TABLE audit_trail (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type   VARCHAR(50) NOT NULL,
    entity_id     UUID        NOT NULL,
    actor_user_id UUID        NULL
                      REFERENCES users (id) ON DELETE SET NULL,
    occurred_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    from_state    VARCHAR(50) NULL,
    to_state      VARCHAR(50) NULL,
    reason        TEXT        NULL,
    metadata      JSONB       NULL
);

-- Imutabilidade: trilha de auditoria é append-only (P-I)
CREATE TRIGGER trg_prevent_mutation_audit_trail
    BEFORE UPDATE OR DELETE ON audit_trail
    FOR EACH ROW EXECUTE FUNCTION fn_prevent_mutation();

-- Índice para consulta de histórico por entidade
CREATE INDEX idx_audit_trail_entity
    ON audit_trail (entity_type, entity_id);

-- Índice para consulta por ator (quem fez o quê)
CREATE INDEX idx_audit_trail_actor
    ON audit_trail (actor_user_id, occurred_at);
