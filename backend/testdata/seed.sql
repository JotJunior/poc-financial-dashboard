-- Seed de dados para testes
-- Ref: tasks.md §0.4.4; quickstart.md §7 Testes
-- ATENÇÃO: executar apenas em ambiente de teste — NUNCA em produção.
-- Depende das migrations 001-009 (FASE 1) estarem aplicadas.

-- Truncar em ordem reversa de FK para limpar estado anterior
-- (migrations de FASE 1 ainda a criar — este seed será expandido)
-- TRUNCATE cascade quando tabelas existirem:
-- TRUNCATE audit_trail, commission_reversals, commissions,
--          order_items, orders, commission_rules, users, vendors
-- CASCADE;

-- Placeholder: seed expandido pela FASE 1 (tasks 1.4.*)
-- Versão atual: apenas confirma que a conexão e as extensões funcionam.

SELECT 'seed.sql carregado com sucesso' AS status;
SELECT count(*) AS extensoes_ativas FROM pg_extension WHERE extname = 'pgcrypto';
