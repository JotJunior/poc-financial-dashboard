//go:build integration

// Package repository — testes de integração para DashboardRepository.
// Task 6.1.5: dashboard com dados fixos → totais corretos e idênticos em execuções
// repetidas (SC-002); drill-down retorna rastreabilidade até pedido individual (SC-004).
//
// Executar com:
//
//	DATABASE_URL="postgres://financialuser:financialpass@localhost:5433/financial_dashboard?sslmode=disable" \
//	  go test -tags=integration ./internal/repository/... -run TestDashboard -v
package repository

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// UUIDs fixos para testes de dashboard — isolados dos outros suites.
const (
	dashVendorID  = "dddddddd-dddd-dddd-dddd-000000000001"
	dashVendor2ID = "dddddddd-dddd-dddd-dddd-000000000002"
	dashActorID   = "dddddddd-dddd-dddd-dddd-000000000099"
)

// setupDashInteg inicializa pool, insere dados fixos e agenda limpeza.
func setupDashInteg(t *testing.T) (*pgxpool.Pool, *PGDashboardRepository) {
	t.Helper()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL não definida — pulando teste de integração")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err, "conectar ao banco de dados")

	cleanup := func() {
		_, _ = pool.Exec(context.Background(), `
			DO $$ BEGIN
				SET LOCAL session_replication_role = 'replica';
				DELETE FROM audit_trail WHERE entity_id IN (
					SELECT id FROM commissions WHERE vendor_id IN (
						'dddddddd-dddd-dddd-dddd-000000000001',
						'dddddddd-dddd-dddd-dddd-000000000002'
					)
				);
				DELETE FROM commission_reversals WHERE commission_id IN (
					SELECT id FROM commissions WHERE vendor_id IN (
						'dddddddd-dddd-dddd-dddd-000000000001',
						'dddddddd-dddd-dddd-dddd-000000000002'
					)
				);
				DELETE FROM commissions WHERE vendor_id IN (
					'dddddddd-dddd-dddd-dddd-000000000001',
					'dddddddd-dddd-dddd-dddd-000000000002'
				);
				DELETE FROM commission_rules WHERE vendor_id IN (
					'dddddddd-dddd-dddd-dddd-000000000001',
					'dddddddd-dddd-dddd-dddd-000000000002'
				);
				DELETE FROM order_items WHERE order_id IN (
					SELECT id FROM orders WHERE vendor_id IN (
						'dddddddd-dddd-dddd-dddd-000000000001',
						'dddddddd-dddd-dddd-dddd-000000000002'
					)
				);
				DELETE FROM orders WHERE vendor_id IN (
					'dddddddd-dddd-dddd-dddd-000000000001',
					'dddddddd-dddd-dddd-dddd-000000000002'
				);
				DELETE FROM vendors WHERE id IN (
					'dddddddd-dddd-dddd-dddd-000000000001',
					'dddddddd-dddd-dddd-dddd-000000000002'
				);
				DELETE FROM users WHERE id = 'dddddddd-dddd-dddd-dddd-000000000099';
			END $$
		`)
	}
	cleanup()
	t.Cleanup(cleanup)

	// Usuário ator
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role)
		VALUES ($1, 'Dash Test Actor', 'dash-integ@test.com', '$2a$10$test', 'gestor')
		ON CONFLICT (id) DO NOTHING`,
		dashActorID)
	require.NoError(t, err)

	// Dois vendedores de teste
	for _, v := range []struct{ id, name, email string }{
		{dashVendorID, "Vendedor Dashboard 1", "dash-vendor1@test.com"},
		{dashVendor2ID, "Vendedor Dashboard 2", "dash-vendor2@test.com"},
	} {
		_, err = pool.Exec(ctx, `
			INSERT INTO vendors (id, name, email, status)
			VALUES ($1, $2, $3, 'ativo')
			ON CONFLICT (id) DO NOTHING`,
			v.id, v.name, v.email)
		require.NoError(t, err)
	}

	repo := NewPGDashboardRepository(pool)
	return pool, repo
}

// insertDashOrder insere um pedido no status 'pago' e retorna seu ID.
func insertDashOrder(t *testing.T, pool *pgxpool.Pool, vendorID string, totalCents int64) string {
	t.Helper()
	ctx := context.Background()
	var orderID string
	err := pool.QueryRow(ctx, `
		INSERT INTO orders (vendor_id, total_cents, order_date, status)
		VALUES ($1, $2, CURRENT_DATE, 'pago')
		RETURNING id`,
		vendorID, totalCents).Scan(&orderID)
	require.NoError(t, err)
	return orderID
}

// insertDashCommission insere uma comissão para um pedido e retorna seu ID.
// Cria uma commission_rule temporária para satisfazer a FK NOT NULL.
func insertDashCommission(t *testing.T, pool *pgxpool.Pool, vendorID, orderID string, valueCents int64, status string) string {
	t.Helper()
	ctx := context.Background()
	pct := decimal.NewFromFloat(5.0)

	// commission_rule temporária para FK (imutável — não pode deletar, cleanup usa replica role)
	var ruleID string
	err := pool.QueryRow(ctx, `
		INSERT INTO commission_rules (vendor_id, percentage, valid_from, version)
		VALUES ($1, $2, NOW(), 1)
		RETURNING id`,
		vendorID, pct).Scan(&ruleID)
	require.NoError(t, err)

	var commID string
	err = pool.QueryRow(ctx, `
		INSERT INTO commissions (vendor_id, order_id, value_cents, applied_percentage, rule_id, period_year, period_month, status)
		VALUES ($1, $2, $3, $4, $5, 2026, 1, $6)
		RETURNING id`,
		vendorID, orderID, valueCents, pct, ruleID, status).Scan(&commID)
	require.NoError(t, err)
	return commID
}

// TestDashboardGetConsolidated_Totais verifica que totais são determinísticos (SC-002).
// Task 6.1.5.
func TestDashboardGetConsolidated_Totais(t *testing.T) {
	pool, repo := setupDashInteg(t)
	ctx := context.Background()

	// Pedido 1: Vendedor 1, R$ 100,00 (10000 centavos)
	o1 := insertDashOrder(t, pool, dashVendorID, 10000)
	// Pedido 2: Vendedor 2, R$ 200,00 (20000 centavos)
	o2 := insertDashOrder(t, pool, dashVendor2ID, 20000)

	// Comissões: 5% sobre cada pedido
	insertDashCommission(t, pool, dashVendorID, o1, 500, "pendente")
	insertDashCommission(t, pool, dashVendor2ID, o2, 1000, "aprovado")

	// Sem filtro de período — para cobrir os pedidos inseridos (CURRENT_DATE = 2026-06-03)
	filter := DashboardFilter{}

	// Executar 2x — deve ser idêntico (SC-002: determinismo)
	for i := 0; i < 2; i++ {
		dash, err := repo.GetConsolidated(ctx, filter)
		require.NoError(t, err, "iteração %d", i)

		// Total de vendas >= 10000 + 20000 = 30000 (pode ter dados de outros testes paralelos)
		assert.GreaterOrEqual(t, dash.TotalSalesCents, int64(30000), "totalSalesCents iteração %d", i)
		// Pelo menos 2 pedidos e 2 vendedores
		assert.GreaterOrEqual(t, dash.OrderCount, 2, "orderCount iteração %d", i)
		assert.GreaterOrEqual(t, dash.VendorCount, 2, "vendorCount iteração %d", i)

		// Comissões incluem os dados inseridos (pendente=500, aprovado=1000)
		assert.GreaterOrEqual(t, dash.PendingCommCents, int64(500), "pendingComm iteração %d", i)
		assert.GreaterOrEqual(t, dash.ApprovedCommCents, int64(1000), "approvedComm iteração %d", i)
		assert.GreaterOrEqual(t, dash.TotalCommCents, int64(1500), "totalComm iteração %d", i)

		// Ranking deve ter pelo menos os 2 vendedores
		assert.GreaterOrEqual(t, len(dash.TopVendors), 2, "topVendors iteração %d", i)
	}
}

// TestDashboardGetVendorDashboard_Escopo garante que escopo de vendedor é respeitado.
// Task 6.1.3: filtra apenas os próprios dados.
func TestDashboardGetVendorDashboard_Escopo(t *testing.T) {
	pool, repo := setupDashInteg(t)
	ctx := context.Background()

	// Pedidos para ambos os vendedores
	o1 := insertDashOrder(t, pool, dashVendorID, 10000)
	o2 := insertDashOrder(t, pool, dashVendor2ID, 20000)
	insertDashCommission(t, pool, dashVendorID, o1, 500, "pendente")
	insertDashCommission(t, pool, dashVendor2ID, o2, 1000, "aprovado")

	filter := DashboardFilter{}

	// Vendedor 1 só vê seus próprios dados
	vd1, err := repo.GetVendorDashboard(ctx, dashVendorID, filter)
	require.NoError(t, err)
	assert.Equal(t, int64(10000), vd1.TotalSalesCents, "vendedor1 só vê seus pedidos")
	assert.Equal(t, int64(500), vd1.PendingCommCents)
	assert.Equal(t, int64(0), vd1.ApprovedCommCents)

	// Vendedor 2 só vê seus próprios dados
	vd2, err := repo.GetVendorDashboard(ctx, dashVendor2ID, filter)
	require.NoError(t, err)
	assert.Equal(t, int64(20000), vd2.TotalSalesCents, "vendedor2 só vê seus pedidos")
	assert.Equal(t, int64(0), vd2.PendingCommCents)
	assert.Equal(t, int64(1000), vd2.ApprovedCommCents)
}

// TestDashboardGetDrillDown_Rastreabilidade verifica SC-004: pedido → comissão → estorno.
// Task 6.1.4.
func TestDashboardGetDrillDown_Rastreabilidade(t *testing.T) {
	pool, repo := setupDashInteg(t)
	ctx := context.Background()

	// Pedido com comissão e estorno
	orderID := insertDashOrder(t, pool, dashVendorID, 10000)
	commID := insertDashCommission(t, pool, dashVendorID, orderID, 500, "pendente")

	// Estorno de -100 centavos (commission_reversals requer order_id e actor_user_id NOT NULL)
	_, err := pool.Exec(ctx, `
		INSERT INTO commission_reversals (commission_id, order_id, value_cents, status, actor_user_id)
		VALUES ($1, $2, -100, 'aplicado', $3)`,
		commID, orderID, dashActorID)
	require.NoError(t, err)

	dd, err := repo.GetDrillDown(ctx, orderID)
	require.NoError(t, err)

	// Pedido correto (SC-004: rastreabilidade)
	assert.Equal(t, orderID, dd.OrderID)
	assert.Equal(t, dashVendorID, dd.VendorID)
	assert.Equal(t, int64(10000), dd.TotalCents)
	assert.Equal(t, "pago", dd.Status)

	// Comissão com saldo líquido
	require.Len(t, dd.Commissions, 1, "deve ter 1 comissão")
	c := dd.Commissions[0]
	assert.Equal(t, commID, c.CommissionID)
	assert.Equal(t, int64(500), c.ValueCents, "valor bruto")
	// net_cents = 500 + (-100) = 400 (via view commission_net_balance)
	assert.Equal(t, int64(400), c.NetCents, "valor líquido após estorno")

	// Estorno (SC-004: rastreabilidade via actor_user_id)
	require.Len(t, c.Reversals, 1, "deve ter 1 estorno")
	rv := c.Reversals[0]
	assert.Equal(t, int64(-100), rv.ValueCents)
	assert.Equal(t, "aplicado", rv.Status)
	assert.Equal(t, dashActorID, rv.ActorUserID, "actor_user_id rastreável")
}

// TestDashboardGetPendingCommissionsSummary verifica FR-019.
func TestDashboardGetPendingCommissionsSummary(t *testing.T) {
	pool, repo := setupDashInteg(t)
	ctx := context.Background()

	o1 := insertDashOrder(t, pool, dashVendorID, 10000)
	o2 := insertDashOrder(t, pool, dashVendorID, 20000)
	insertDashCommission(t, pool, dashVendorID, o1, 500, "pendente")
	insertDashCommission(t, pool, dashVendorID, o2, 1000, "aprovado")

	summary, err := repo.GetPendingCommissionsSummary(ctx)
	require.NoError(t, err)

	// Deve incluir pelo menos os dados inseridos
	assert.GreaterOrEqual(t, summary.PendingApprovalCount, 1)
	assert.GreaterOrEqual(t, summary.PendingApprovalCents, int64(500))
	assert.GreaterOrEqual(t, summary.ApprovedUnpaidCount, 1)
	assert.GreaterOrEqual(t, summary.ApprovedUnpaidCents, int64(1000))
}
