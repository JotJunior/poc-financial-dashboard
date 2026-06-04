-- seed_demo.sql — Dados de DEMONSTRAÇÃO navegáveis (vendedores, regras,
-- pedidos, comissões). Objetivo: que a aplicação não fique vazia ao explorar.
--
-- DATAS RELATIVAS A CURRENT_DATE: como o banco é resetado diariamente, os
-- pedidos são distribuídos no MÊS ATUAL + 2 meses anteriores. Assim o dashboard
-- (que abre no mês corrente) sempre aparece populado, em qualquer dia de reset.
--
-- Consistência: as comissões são geradas por SELECT dos pedidos 'pago',
-- espelhando a apuração do backend — value_cents = ROUND(total_cents * % / 100)
-- (half-up), período derivado do order_date, regra vigente na data do pedido.
-- Logo, "Apurar" pela UI é idempotente sobre estes dados (UNIQUE order_id).
--
-- Status das comissões por idade (demonstra os 3 buckets do dashboard):
--   mês atual    -> 'pendente'   (aguardando aprovação)
--   mês anterior -> 'aprovado'   (a pagar)
--   2 meses atrás-> 'pago'
--
-- Pré-requisitos: migrations 001–011 + seed_users.sql. Idempotente.
-- ATENÇÃO: dados de DEMONSTRAÇÃO — NUNCA usar em produção.

BEGIN;

-- ─── Vendedores ──────────────────────────────────────────────────────────────
-- O vendedor e2e00001 já existe (seed_users) e é o vínculo do login Vendedor
-- (vendedor-user-e2e@test.com) — por isso recebe dados próprios para navegar.
INSERT INTO vendors (id, name, email, status, created_at) VALUES
  ('5eed0001-0000-0000-0000-000000000002', 'Ana Souza',    'ana.souza@demo.test',    'ativo',   now() - interval '500 days'),
  ('5eed0001-0000-0000-0000-000000000003', 'Bruno Lima',   'bruno.lima@demo.test',   'ativo',   now() - interval '480 days'),
  ('5eed0001-0000-0000-0000-000000000004', 'Carla Mendes', 'carla.mendes@demo.test', 'ativo',   now() - interval '400 days'),
  ('5eed0001-0000-0000-0000-000000000005', 'Diego Rocha',  'diego.rocha@demo.test',  'inativo', now() - interval '300 days')
ON CONFLICT (id) DO NOTHING;

-- ─── Regras de comissão (vigentes desde 12 meses atrás; valid_to NULL) ───────
INSERT INTO commission_rules (id, vendor_id, percentage, valid_from, valid_to, version) VALUES
  ('5eed0002-0000-0000-0000-000000000001', 'e2e00001-0000-0000-0000-000000000001',  8.0000, (date_trunc('month', CURRENT_DATE) - interval '12 months')::date, NULL, 1),
  ('5eed0002-0000-0000-0000-000000000002', '5eed0001-0000-0000-0000-000000000002',  5.0000, (date_trunc('month', CURRENT_DATE) - interval '12 months')::date, NULL, 1),
  ('5eed0002-0000-0000-0000-000000000003', '5eed0001-0000-0000-0000-000000000003', 10.0000, (date_trunc('month', CURRENT_DATE) - interval '12 months')::date, NULL, 1),
  ('5eed0002-0000-0000-0000-000000000004', '5eed0001-0000-0000-0000-000000000004',  6.5000, (date_trunc('month', CURRENT_DATE) - interval '12 months')::date, NULL, 1)
ON CONFLICT (id) DO NOTHING;

-- ─── Pedidos (datas relativas) ───────────────────────────────────────────────
-- Helpers de data (inline):
--   mês atual  dia D : LEAST(date_trunc('month',CURRENT_DATE)::date + (D-1), CURRENT_DATE)
--   mês -1     dia D : (date_trunc('month',CURRENT_DATE) - interval '1 month')::date + (D-1)
--   mês -2     dia D : (date_trunc('month',CURRENT_DATE) - interval '2 months')::date + (D-1)
-- 'pago' gera comissão; 'confirmado'/'cancelado' não.
INSERT INTO orders (id, vendor_id, total_cents, order_date, status, paid_at) VALUES
  -- Vendedor E2E Teste (8%)
  ('5eed0003-0000-0000-0000-000000000001', 'e2e00001-0000-0000-0000-000000000001', 250000, (date_trunc('month',CURRENT_DATE) - interval '2 months')::date + 11, 'pago',       ((date_trunc('month',CURRENT_DATE) - interval '2 months')::date + 13)::timestamptz),
  ('5eed0003-0000-0000-0000-000000000002', 'e2e00001-0000-0000-0000-000000000001', 480000, (date_trunc('month',CURRENT_DATE) - interval '1 month')::date + 14,  'pago',       ((date_trunc('month',CURRENT_DATE) - interval '1 month')::date + 16)::timestamptz),
  ('5eed0003-0000-0000-0000-000000000003', 'e2e00001-0000-0000-0000-000000000001', 360000, date_trunc('month',CURRENT_DATE)::date,                                'pago',       date_trunc('month',CURRENT_DATE)::date::timestamptz),
  ('5eed0003-0000-0000-0000-000000000004', 'e2e00001-0000-0000-0000-000000000001', 150000, CURRENT_DATE,                                                          'confirmado', NULL),
  -- Ana Souza (5%)
  ('5eed0003-0000-0000-0000-000000000005', '5eed0001-0000-0000-0000-000000000002', 600000, (date_trunc('month',CURRENT_DATE) - interval '2 months')::date + 21, 'pago',       ((date_trunc('month',CURRENT_DATE) - interval '2 months')::date + 24)::timestamptz),
  ('5eed0003-0000-0000-0000-000000000006', '5eed0001-0000-0000-0000-000000000002', 320000, (date_trunc('month',CURRENT_DATE) - interval '1 month')::date + 7,   'pago',       ((date_trunc('month',CURRENT_DATE) - interval '1 month')::date + 9)::timestamptz),
  ('5eed0003-0000-0000-0000-000000000007', '5eed0001-0000-0000-0000-000000000002', 210000, LEAST(date_trunc('month',CURRENT_DATE)::date + 1, CURRENT_DATE),      'pago',       LEAST(date_trunc('month',CURRENT_DATE)::date + 1, CURRENT_DATE)::timestamptz),
  -- Bruno Lima (10%)
  ('5eed0003-0000-0000-0000-000000000008', '5eed0001-0000-0000-0000-000000000003', 900000, (date_trunc('month',CURRENT_DATE) - interval '2 months')::date + 27, 'pago',       ((date_trunc('month',CURRENT_DATE) - interval '2 months')::date + 29)::timestamptz),
  ('5eed0003-0000-0000-0000-000000000009', '5eed0001-0000-0000-0000-000000000003', 450000, (date_trunc('month',CURRENT_DATE) - interval '1 month')::date + 24,  'pago',       ((date_trunc('month',CURRENT_DATE) - interval '1 month')::date + 26)::timestamptz),
  ('5eed0003-0000-0000-0000-000000000010', '5eed0001-0000-0000-0000-000000000003', 275000, LEAST(date_trunc('month',CURRENT_DATE)::date + 2, CURRENT_DATE),      'pago',       LEAST(date_trunc('month',CURRENT_DATE)::date + 2, CURRENT_DATE)::timestamptz),
  ('5eed0003-0000-0000-0000-000000000011', '5eed0001-0000-0000-0000-000000000003', 120000, (date_trunc('month',CURRENT_DATE) - interval '1 month')::date + 1,   'cancelado',  NULL),
  -- Carla Mendes (6.5%)
  ('5eed0003-0000-0000-0000-000000000012', '5eed0001-0000-0000-0000-000000000004', 540000, (date_trunc('month',CURRENT_DATE) - interval '2 months')::date + 17, 'pago',       ((date_trunc('month',CURRENT_DATE) - interval '2 months')::date + 19)::timestamptz),
  ('5eed0003-0000-0000-0000-000000000013', '5eed0001-0000-0000-0000-000000000004', 380000, (date_trunc('month',CURRENT_DATE) - interval '1 month')::date + 15,  'pago',       ((date_trunc('month',CURRENT_DATE) - interval '1 month')::date + 18)::timestamptz),
  ('5eed0003-0000-0000-0000-000000000014', '5eed0001-0000-0000-0000-000000000004',  90000, CURRENT_DATE,                                                          'confirmado', NULL)
ON CONFLICT (id) DO NOTHING;

-- ─── Itens de pedido (1 item por pedido; line_total = total do pedido) ────────
INSERT INTO order_items (id, order_id, description, quantity, unit_price_cents, line_total_cents) VALUES
  ('5eed0004-0000-0000-0000-000000000001', '5eed0003-0000-0000-0000-000000000001', 'Plano Anual — Licença Pro',  1, 250000, 250000),
  ('5eed0004-0000-0000-0000-000000000002', '5eed0003-0000-0000-0000-000000000002', 'Consultoria de implantação', 1, 480000, 480000),
  ('5eed0004-0000-0000-0000-000000000003', '5eed0003-0000-0000-0000-000000000003', 'Pacote de treinamento',      1, 360000, 360000),
  ('5eed0004-0000-0000-0000-000000000004', '5eed0003-0000-0000-0000-000000000004', 'Add-on de relatórios',       1, 150000, 150000),
  ('5eed0004-0000-0000-0000-000000000005', '5eed0003-0000-0000-0000-000000000005', 'Plano Enterprise — Anual',   1, 600000, 600000),
  ('5eed0004-0000-0000-0000-000000000006', '5eed0003-0000-0000-0000-000000000006', 'Licenças adicionais (lote)', 1, 320000, 320000),
  ('5eed0004-0000-0000-0000-000000000007', '5eed0003-0000-0000-0000-000000000007', 'Pacote de suporte anual',    1, 210000, 210000),
  ('5eed0004-0000-0000-0000-000000000008', '5eed0003-0000-0000-0000-000000000008', 'Contrato corporativo',       1, 900000, 900000),
  ('5eed0004-0000-0000-0000-000000000009', '5eed0003-0000-0000-0000-000000000009', 'Renovação anual',            1, 450000, 450000),
  ('5eed0004-0000-0000-0000-000000000010', '5eed0003-0000-0000-0000-000000000010', 'Módulo de integração',       1, 275000, 275000),
  ('5eed0004-0000-0000-0000-000000000011', '5eed0003-0000-0000-0000-000000000011', 'Pedido cancelado',           1, 120000, 120000),
  ('5eed0004-0000-0000-0000-000000000012', '5eed0003-0000-0000-0000-000000000012', 'Plano Anual — Licença Pro',  1, 540000, 540000),
  ('5eed0004-0000-0000-0000-000000000013', '5eed0003-0000-0000-0000-000000000013', 'Expansão de assentos',       1, 380000, 380000),
  ('5eed0004-0000-0000-0000-000000000014', '5eed0003-0000-0000-0000-000000000014', 'Suporte premium (mensal)',   1,  90000,  90000)
ON CONFLICT (id) DO NOTHING;

-- ─── Comissões (geradas dos pedidos 'pago' — espelha a apuração) ─────────────
-- value_cents = ROUND(total_cents * percentage / 100) (half-up, igual ao backend).
-- período = mês/ano do order_date. status por idade (pendente/aprovado/pago).
INSERT INTO commissions
  (id, order_id, vendor_id, value_cents, applied_percentage, rule_id, period_year, period_month, status)
SELECT
  gen_random_uuid(),
  o.id,
  o.vendor_id,
  ROUND(o.total_cents * r.percentage / 100)::bigint,
  r.percentage,
  r.id,
  EXTRACT(YEAR  FROM o.order_date)::int,
  EXTRACT(MONTH FROM o.order_date)::int,
  CASE
    WHEN date_trunc('month', o.order_date) = date_trunc('month', CURRENT_DATE)
      THEN 'pendente'
    WHEN date_trunc('month', o.order_date) = date_trunc('month', CURRENT_DATE - interval '1 month')
      THEN 'aprovado'
    ELSE 'pago'
  END
FROM orders o
JOIN commission_rules r
  ON r.vendor_id = o.vendor_id
 AND r.valid_from <= o.order_date
 AND (r.valid_to IS NULL OR r.valid_to >= o.order_date)
WHERE o.status = 'pago'
  AND o.id::text LIKE '5eed0003-%'   -- apenas pedidos deste seed
ON CONFLICT (order_id) DO NOTHING;

COMMIT;
