-- Migration 001: Schema base inicial
-- Cria extensão pgcrypto para gen_random_uuid() e tipos base.
-- Tabelas completas criadas nas migrations 002-009 (FASE 1).
-- Ref: data-model.md; tasks.md §1.4; constitution P-III (sem float)

-- Extensão para UUID v4
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Função utilitária: impedir UPDATE/DELETE (trilha append-only — constitution P-I)
-- Usada como trigger em tabelas financeiras imutáveis.
CREATE OR REPLACE FUNCTION fn_prevent_mutation()
RETURNS TRIGGER AS $$
BEGIN
  RAISE EXCEPTION 'Operacao % proibida em tabela imutavel % (constitution P-I — auditabilidade)', TG_OP, TG_TABLE_NAME;
END;
$$ LANGUAGE plpgsql;
