-- seed_users.sql — usuários de demonstração para o ambiente de desenvolvimento.
--
-- Cria um vendedor-base e os 3 atores do sistema (Gestor, Financeiro, Vendedor),
-- todos com a MESMA senha de demonstração para facilitar a exploração da UI.
--
--   Senha (todos): E2ETest@2026!
--   Gestor:     gestor-e2e@test.com         (papel: gestor)
--   Financeiro: financeiro-e2e@test.com     (papel: financeiro)
--   Vendedor:   vendedor-user-e2e@test.com  (papel: vendedor)
--
-- O password_hash é Argon2id (m=64MB, t=3, p=4), o mesmo formato que o backend
-- gera/valida (constitution P-IV / dec-052).
--
-- ATENÇÃO: dados de DEMONSTRAÇÃO — NUNCA usar em produção. Troque as senhas.
-- Pré-requisito: migrations aplicadas (make migrate-up). Idempotente (ON CONFLICT).
--
-- Uso:
--   docker exec -i financial-dashboard-db \
--     psql -U financialuser -d financial_dashboard < backend/testdata/seed_users.sql

BEGIN;

-- Vendedor-base (FK exigida pelo usuário de papel "vendedor").
INSERT INTO vendors (id, name, email, status, created_at) VALUES
  ('e2e00001-0000-0000-0000-000000000001', 'Vendedor E2E Teste', 'vendedor-e2e@test.com', 'ativo', now())
ON CONFLICT (id) DO NOTHING;

-- Atores. Hash Argon2id de "E2ETest@2026!".
INSERT INTO users (id, name, email, password_hash, role, vendor_id) VALUES
  ('e2e00002-0000-0000-0000-000000000001', 'Gestor E2E',        'gestor-e2e@test.com',        '$argon2id$v=19$m=65536,t=3,p=4$QrvRQ7BTJorn8sSVPRGH9Q$91Fly0wr3NiIGuoWNl6RH0chBUvuu7GwmFRg3NHLfN0', 'gestor',     NULL),
  ('e2e00002-0000-0000-0000-000000000002', 'Financeiro E2E',    'financeiro-e2e@test.com',    '$argon2id$v=19$m=65536,t=3,p=4$QrvRQ7BTJorn8sSVPRGH9Q$91Fly0wr3NiIGuoWNl6RH0chBUvuu7GwmFRg3NHLfN0', 'financeiro', NULL),
  ('e2e00002-0000-0000-0000-000000000003', 'Vendedor User E2E', 'vendedor-user-e2e@test.com', '$argon2id$v=19$m=65536,t=3,p=4$QrvRQ7BTJorn8sSVPRGH9Q$91Fly0wr3NiIGuoWNl6RH0chBUvuu7GwmFRg3NHLfN0', 'vendedor',   'e2e00001-0000-0000-0000-000000000001')
ON CONFLICT (id) DO NOTHING;

COMMIT;
