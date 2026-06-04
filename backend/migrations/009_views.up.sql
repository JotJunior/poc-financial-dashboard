-- Migration 009: Views e índices de performance
-- Ref: data-model.md §views; tasks.md §1.4.8/1.4.9
-- Constitution P-III: net_cents é BIGINT (soma de BIGINT) — zero float

-- View: saldo líquido de comissão (valor original + estornos acumulados)
-- value_cents (positivo) + SUM(reversals.value_cents) (negativo ou zero) = saldo líquido
CREATE VIEW commission_net_balance AS
SELECT
    c.id                                                                AS commission_id,
    c.vendor_id,
    c.order_id,
    c.period_year,
    c.period_month,
    c.status,
    c.value_cents + COALESCE(SUM(cr.value_cents), 0)                   AS net_cents,
    c.value_cents                                                       AS gross_cents,
    COALESCE(SUM(cr.value_cents), 0)                                   AS total_reversal_cents
FROM commissions c
LEFT JOIN commission_reversals cr ON cr.commission_id = c.id
GROUP BY
    c.id,
    c.vendor_id,
    c.order_id,
    c.period_year,
    c.period_month,
    c.status,
    c.value_cents;
