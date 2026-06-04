//go:build integration

// Package integration — FASE 9 integration tests.
// Tasks 9.1.1-9.1.6: Testes de integração backend com banco real (PostgreSQL).
//
//   - 9.1.1: CRUD de vendedores + histórico de regras; ciclo completo de pedido
//     (rascunho→confirmado→pago→cancelado + CHK081)
//   - 9.1.2: Apuração: 1 vendedor 10% + 3 pedidos R$1.000 = R$300 exatos; reprocessar → 0 duplicatas (SC-003)
//   - 9.1.3: Escopo RBAC: Vendedor tentando acessar dados de outro → 403 em TODOS os endpoints relevantes (SC-005)
//   - 9.1.4: Schema float: query information_schema.columns → 0 colunas float em campos monetários (SC-007/CHK063)
//   - 9.1.5: audit_trail: toda transição de estado registra ator + timestamp + from_state + to_state (SC-006)
//   - 9.1.6: LGPD anonimização: PII substituída + audit_trail gravada (CHK078) + registros financeiros preservados
//
// Executar com:
//
//	DATABASE_URL="postgres://financialuser:financialpass@localhost:5433/financial_dashboard?sslmode=disable" \
//	  go test -tags=integration ./internal/integration/... -v -timeout 120s
package integration

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"financial-dashboard/backend/internal/domain"
	"financial-dashboard/backend/internal/repository"
	"financial-dashboard/backend/internal/service"
)

// ─── Helpers ─────────────────────────────────────────────────────────────────

func fase9Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL não definida — pulando teste de integração")
	}
	pool, err := pgxpool.New(context.Background(), dbURL)
	require.NoError(t, err, "conectar ao banco de dados")
	t.Cleanup(func() { pool.Close() })
	return pool
}

// execBypass executa SQL com session_replication_role='replica' para contornar
// triggers de imutabilidade (apenas em testes — jamais em produção).
func execBypass(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		return // cleanup best-effort
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	_, _ = tx.Exec(ctx, "SET LOCAL session_replication_role = 'replica'")
	_, _ = tx.Exec(ctx, sql, args...)
	_ = tx.Commit(ctx)
}

// insertUser insere usuário de teste (idempotente).
func insertUser(t *testing.T, pool *pgxpool.Pool, id, name, email, role, vendorID string) {
	t.Helper()
	ctx := context.Background()
	if vendorID == "" {
		_, err := pool.Exec(ctx, `
			INSERT INTO users (id, name, email, password_hash, role)
			VALUES ($1, $2, $3, '$argon2id$v=19$m=65536,t=3,p=4$dGVzdA$dGVzdA', $4)
			ON CONFLICT (id) DO NOTHING
		`, id, name, email, role)
		require.NoError(t, err, "inserir usuário %s", email)
	} else {
		_, err := pool.Exec(ctx, `
			INSERT INTO users (id, name, email, password_hash, role, vendor_id)
			VALUES ($1, $2, $3, '$argon2id$v=19$m=65536,t=3,p=4$dGVzdA$dGVzdA', $4, $5)
			ON CONFLICT (id) DO NOTHING
		`, id, name, email, role, vendorID)
		require.NoError(t, err, "inserir usuário vendedor %s", email)
	}
}

// insertVendor insere vendedor de teste (idempotente).
func insertVendor(t *testing.T, pool *pgxpool.Pool, id, name, email string) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO vendors (id, name, email, status)
		VALUES ($1, $2, $3, 'ativo')
		ON CONFLICT (id) DO NOTHING
	`, id, name, email)
	require.NoError(t, err, "inserir vendedor %s", email)
}

// nullBlocklist é uma implementação no-op de BlocklistRevoker para testes.
type nullBlocklist struct{}

func (n *nullBlocklist) RevokeAllForVendor(_ context.Context, _ string, _ string) error { return nil }

// strPtr retorna ponteiro para uma string.
func strPtr(s string) *string { return &s }

// intPtr retorna ponteiro para um int.
func intPtr(i int) *int { return &i }

// ─── 9.1.1 — Ciclo completo de pedido + CRUD de vendedor ─────────────────────

// TestFase9_DBConnectivity verifica que o banco está acessível.
func TestFase9_DBConnectivity(t *testing.T) {
	pool := fase9Pool(t)
	var one int
	err := pool.QueryRow(context.Background(), "SELECT 1").Scan(&one)
	require.NoError(t, err, "deve conectar ao banco e fazer query básica")
	assert.Equal(t, 1, one)
}

// TestFase9_CicloPedidoCompleto (9.1.1): rascunho→confirmado→pago→cancelado + audit_trail.
func TestFase9_CicloPedidoCompleto(t *testing.T) {
	pool := fase9Pool(t)
	ctx := context.Background()

	const (
		vendorID = "f9a10001-0000-0000-0000-000000000001"
		actorID  = "f9a10001-0000-0000-0000-000000000002"
	)

	t.Cleanup(func() {
		execBypass(t, pool, `DELETE FROM audit_trail WHERE entity_id IN (SELECT id FROM orders WHERE vendor_id = $1)`, vendorID)
		execBypass(t, pool, `DELETE FROM order_items WHERE order_id IN (SELECT id FROM orders WHERE vendor_id = $1)`, vendorID)
		execBypass(t, pool, `DELETE FROM orders WHERE vendor_id = $1`, vendorID)
		execBypass(t, pool, `DELETE FROM commission_rules WHERE vendor_id = $1`, vendorID)
		execBypass(t, pool, `DELETE FROM vendors WHERE id = $1`, vendorID)
		execBypass(t, pool, `DELETE FROM users WHERE id = $1`, actorID)
	})

	insertVendor(t, pool, vendorID, "Vendedor Ciclo F9", "f9a-ciclo@test.com")
	insertUser(t, pool, actorID, "Gestor F9", "f9a-gestor@test.com", "gestor", "")

	// Criar regra de comissão
	ruleRepo := repository.NewPGCommissionRuleRepository(pool)
	rule, err := ruleRepo.CreateRule(ctx, vendorID, decimal.NewFromFloat(10.0))
	require.NoError(t, err, "criar regra de comissão")
	assert.NotEmpty(t, rule.ID)
	assert.Equal(t, 1, rule.Version, "primeira versão da regra deve ser 1")

	// Criar pedido
	orderRepo := repository.NewPGOrderRepository(pool)
	order, err := orderRepo.Create(ctx, repository.CreateOrderReq{
		VendorID:   vendorID,
		TotalCents: 100000, // R$ 1.000,00
		OrderDate:  time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
		Items: []repository.CreateOrderItemReq{
			{Description: "Produto Ciclo", Quantity: 1, UnitPriceCents: 100000, LineTotalCents: 100000},
		},
	})
	require.NoError(t, err, "criar pedido")
	assert.Equal(t, domain.OrderStatusRascunho, order.Status)

	// rascunho → confirmado
	confirmed, err := orderRepo.Transition(ctx, order.ID, domain.OrderStatusConfirmado, actorID)
	require.NoError(t, err, "transição rascunho→confirmado")
	assert.Equal(t, domain.OrderStatusConfirmado, confirmed.Status)

	// confirmado → pago
	paid, err := orderRepo.Transition(ctx, order.ID, domain.OrderStatusPago, actorID)
	require.NoError(t, err, "transição confirmado→pago")
	assert.Equal(t, domain.OrderStatusPago, paid.Status)
	assert.NotNil(t, paid.PaidAt, "paid_at deve ser preenchido")

	// pago → cancelado
	cancelled, err := orderRepo.Transition(ctx, order.ID, domain.OrderStatusCancelado, actorID)
	require.NoError(t, err, "transição pago→cancelado")
	assert.Equal(t, domain.OrderStatusCancelado, cancelled.Status)

	// Verificar audit_trail: 3 transições (SC-006)
	var auditCount int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_trail WHERE entity_type = 'order' AND entity_id = $1`,
		order.ID).Scan(&auditCount)
	require.NoError(t, err)
	assert.Equal(t, 3, auditCount, "devem haver 3 registros de audit_trail")

	// Transição inválida: cancelado → rascunho deve falhar
	_, err = orderRepo.Transition(ctx, order.ID, domain.OrderStatusRascunho, actorID)
	require.Error(t, err, "transição inválida cancelado→rascunho deve retornar erro")
}

// TestFase9_VendorHistoricoRegras (9.1.1): CRUD de vendedor + histórico de regras de comissão.
func TestFase9_VendorHistoricoRegras(t *testing.T) {
	pool := fase9Pool(t)
	ctx := context.Background()

	const (
		vendorID = "f9a10003-0000-0000-0000-000000000001"
		actorID  = "f9a10003-0000-0000-0000-000000000002"
	)

	t.Cleanup(func() {
		execBypass(t, pool, `DELETE FROM commission_rules WHERE vendor_id = $1`, vendorID)
		execBypass(t, pool, `DELETE FROM vendors WHERE id = $1`, vendorID)
		execBypass(t, pool, `DELETE FROM users WHERE id = $1`, actorID)
	})

	insertUser(t, pool, actorID, "Gestor Historico", "f9a-historico-gestor@test.com", "gestor", "")

	ruleRepo := repository.NewPGCommissionRuleRepository(pool)
	vendorRepo := repository.NewPGVendorRepository(pool)
	svc := service.NewVendorService(vendorRepo, ruleRepo, &nullBlocklist{})

	// Criar vendedor com regra inicial 8%
	created, err := svc.CreateVendor(ctx, service.CreateVendorReq{
		Name:                 "Vendedor Historico F9",
		Email:                "f9a-historico@test.com",
		CommissionPercentage: decimal.NewFromFloat(8.0),
	}, "gestor")
	require.NoError(t, err, "criar vendedor com regra inicial")
	assert.Equal(t, "Vendedor Historico F9", created.Name)

	// Capturar o ID real criado pelo service
	createdID := created.ID

	// Update: nova regra de comissão 12% (versão 2)
	newPct := decimal.NewFromFloat(12.0)
	_, err = svc.UpdateVendor(ctx, createdID, service.UpdateVendorReq{
		Percentage: &newPct,
	}, "gestor")
	require.NoError(t, err, "atualizar comissão para versão 2")

	// Listar regras: deve ter 2 versões
	rules, err := ruleRepo.ListByVendor(ctx, createdID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(rules), 2, "deve ter pelo menos 2 versões de regra de comissão")

	// Desativar vendedor
	err = svc.DeactivateVendor(ctx, createdID, "gestor")
	require.NoError(t, err, "desativar vendedor")

	// Verificar status = inativo
	v, err := vendorRepo.FindByID(ctx, createdID)
	require.NoError(t, err)
	assert.Equal(t, repository.VendorStatusInativo, v.Status, "vendedor deve estar inativo após desativação")

	// Cleanup: remover o vendedor criado dinamicamente
	t.Cleanup(func() {
		execBypass(t, pool, `DELETE FROM commission_rules WHERE vendor_id = $1`, createdID)
		execBypass(t, pool, `DELETE FROM vendors WHERE id = $1`, createdID)
	})
}

// ─── 9.1.2 — Apuração: US3 Independent Test ──────────────────────────────────

// TestFase9_Apuracao_US3_1VendorTresPedidosR300 (9.1.2):
// 1 vendedor 10% + 3 pedidos R$1.000 = R$300 exatos. Reprocessar → 0 duplicatas (SC-003).
func TestFase9_Apuracao_US3_1VendorTresPedidosR300(t *testing.T) {
	pool := fase9Pool(t)
	ctx := context.Background()

	const (
		vendorID = "f9a20001-0000-0000-0000-000000000001"
		actorID  = "f9a20001-0000-0000-0000-000000000002"
		// Usar mês de agosto (8) de 2026: ordem_date >= valid_from (CURRENT_DATE ~= junho 2026).
		year  = 2026
		month = 8
	)

	t.Cleanup(func() {
		execBypass(t, pool, `DELETE FROM commissions WHERE vendor_id = $1`, vendorID)
		execBypass(t, pool, `DELETE FROM audit_trail WHERE entity_id IN (SELECT id FROM orders WHERE vendor_id = $1)`, vendorID)
		execBypass(t, pool, `DELETE FROM order_items WHERE order_id IN (SELECT id FROM orders WHERE vendor_id = $1)`, vendorID)
		execBypass(t, pool, `DELETE FROM orders WHERE vendor_id = $1`, vendorID)
		execBypass(t, pool, `DELETE FROM commission_rules WHERE vendor_id = $1`, vendorID)
		execBypass(t, pool, `DELETE FROM vendors WHERE id = $1`, vendorID)
		execBypass(t, pool, `DELETE FROM users WHERE id = $1`, actorID)
	})

	insertVendor(t, pool, vendorID, "Vendedor Apuracao US3", "f9a-apuracao-us3@test.com")
	insertUser(t, pool, actorID, "Gestor Apuracao", "f9a-apuracao-gestor@test.com", "gestor", "")

	// Criar regra 10%
	ruleRepo := repository.NewPGCommissionRuleRepository(pool)
	_, err := ruleRepo.CreateRule(ctx, vendorID, decimal.NewFromFloat(10.0))
	require.NoError(t, err)

	// Criar 3 pedidos de R$1.000 cada (transicionados para pago)
	orderRepo := repository.NewPGOrderRepository(pool)
	for i := 0; i < 3; i++ {
		order, err := orderRepo.Create(ctx, repository.CreateOrderReq{
			VendorID:   vendorID,
			TotalCents: 100000, // R$ 1.000,00
			OrderDate:  time.Date(year, time.Month(month), i+1, 0, 0, 0, 0, time.UTC),
		})
		require.NoError(t, err, "criar pedido %d", i+1)
		_, err = orderRepo.Transition(ctx, order.ID, domain.OrderStatusConfirmado, actorID)
		require.NoError(t, err)
		_, err = orderRepo.Transition(ctx, order.ID, domain.OrderStatusPago, actorID)
		require.NoError(t, err)
	}

	// Apurar
	commRepo := repository.NewPGCommissionRepository(pool)
	apurSvc := service.NewApurationService(orderRepo, ruleRepo, commRepo)

	result, err := apurSvc.ApurateMonth(ctx, year, month, "gestor")
	require.NoError(t, err, "apurar mês")
	assert.Equal(t, 3, result.Calculated, "deve calcular 3 comissões")
	assert.Equal(t, 0, result.Skipped, "zero duplicatas na primeira apuração")

	// Verificar valor: 3 × R$1.000 × 10% = R$300 = 30.000 centavos
	comms, err := commRepo.FindByPeriod(ctx, repository.CommissionFilter{
		VendorID: strPtr(vendorID),
		PeriodYear: intPtr(year),
		PeriodMonth: intPtr(month),
	})
	require.NoError(t, err)
	require.Len(t, comms, 3, "deve haver exatamente 3 comissões")

	var totalCents int64
	for _, c := range comms {
		totalCents += c.ValueCents
		assert.True(t, c.AppliedPercentage.Equal(decimal.NewFromFloat(10.0)),
			"percentual aplicado deve ser 10%%, got: %s", c.AppliedPercentage.String())
	}
	assert.Equal(t, int64(30000), totalCents,
		"total: 3 × R$1.000 × 10%% = R$300 = 30.000 centavos")

	// Reprocessar → idempotente (SC-003)
	result2, err := apurSvc.ApurateMonth(ctx, year, month, "gestor")
	require.NoError(t, err, "reapurar mês (SC-003)")
	assert.Equal(t, 0, result2.Calculated, "reprocessamento não deve inserir duplicatas")
	assert.Equal(t, 3, result2.Skipped, "3 comissões devem ser puladas como duplicatas")

	// Confirmar que ainda há exatamente 3 comissões
	comms2, err := commRepo.FindByPeriod(ctx, repository.CommissionFilter{
		VendorID: strPtr(vendorID),
		PeriodYear: intPtr(year),
		PeriodMonth: intPtr(month),
	})
	require.NoError(t, err)
	assert.Len(t, comms2, 3, "deve haver EXATAMENTE 3 comissões após reprocessamento (sem duplicatas)")
}

// ─── 9.1.3 — Escopo RBAC: Vendedor não acessa dados de outro (SC-005) ─────────

// TestFase9_RBAC_VendedorNaoAcessaDadosDeOutro (9.1.3): verifica escopo RBAC
// no nível de service para cada operação que pode vazar dados cross-vendor.
func TestFase9_RBAC_VendedorNaoAcessaDadosDeOutro(t *testing.T) {
	pool := fase9Pool(t)
	ctx := context.Background()

	const (
		vendor1ID = "f9a30001-0000-0000-0000-000000000001"
		vendor2ID = "f9a30001-0000-0000-0000-000000000002"
		user1ID   = "f9a30001-0000-0000-0000-000000000003"
		gestorID  = "f9a30001-0000-0000-0000-000000000005"
	)

	t.Cleanup(func() {
		execBypass(t, pool, `DELETE FROM commissions WHERE vendor_id IN ($1, $2)`, vendor1ID, vendor2ID)
		execBypass(t, pool, `DELETE FROM audit_trail WHERE entity_id IN (SELECT id FROM orders WHERE vendor_id IN ($1, $2))`, vendor1ID, vendor2ID)
		execBypass(t, pool, `DELETE FROM order_items WHERE order_id IN (SELECT id FROM orders WHERE vendor_id IN ($1, $2))`, vendor1ID, vendor2ID)
		execBypass(t, pool, `DELETE FROM orders WHERE vendor_id IN ($1, $2)`, vendor1ID, vendor2ID)
		execBypass(t, pool, `DELETE FROM commission_rules WHERE vendor_id IN ($1, $2)`, vendor1ID, vendor2ID)
		execBypass(t, pool, `DELETE FROM users WHERE id IN ($1, $2)`, user1ID, gestorID)
		execBypass(t, pool, `DELETE FROM vendors WHERE id IN ($1, $2)`, vendor1ID, vendor2ID)
	})

	insertVendor(t, pool, vendor1ID, "Vendedor RBAC 1", "f9a-rbac-v1@test.com")
	insertVendor(t, pool, vendor2ID, "Vendedor RBAC 2", "f9a-rbac-v2@test.com")
	insertUser(t, pool, user1ID, "User Vendedor 1", "f9a-rbac-user1@test.com", "vendedor", vendor1ID)
	insertUser(t, pool, gestorID, "Gestor RBAC", "f9a-rbac-gestor@test.com", "gestor", "")

	vendorRepo := repository.NewPGVendorRepository(pool)
	ruleRepo := repository.NewPGCommissionRuleRepository(pool)
	svc := service.NewVendorService(vendorRepo, ruleRepo, &nullBlocklist{})

	// A — Vendedor 1 NÃO pode ver dados do vendedor 2
	_, err := svc.GetVendor(ctx, vendor2ID, "vendedor", vendor1ID)
	require.Error(t, err, "vendedor 1 não deve ver dados do vendedor 2 (SC-005)")

	// B — Vendedor pode ver seus próprios dados
	_, err = svc.GetVendor(ctx, vendor1ID, "vendedor", vendor1ID)
	require.NoError(t, err, "vendedor deve poder ver seus próprios dados")

	// C — Vendedor NÃO pode listar todos os vendedores
	_, err = svc.ListVendors(ctx, repository.VendorFilter{}, "vendedor")
	require.Error(t, err, "vendedor não pode listar todos os vendedores")

	// D — Gestor pode listar todos
	vendors, err := svc.ListVendors(ctx, repository.VendorFilter{}, "gestor")
	require.NoError(t, err, "gestor pode listar todos os vendedores")
	assert.GreaterOrEqual(t, len(vendors), 2, "gestor deve ver pelo menos 2 vendedores")

	// E — Vendedor NÃO pode criar vendedor
	_, err = svc.CreateVendor(ctx, service.CreateVendorReq{
		Name:                 "Tentativa Proibida",
		Email:                "f9a-proibido@test.com",
		CommissionPercentage: decimal.NewFromFloat(5.0),
	}, "vendedor")
	require.Error(t, err, "vendedor não pode criar vendedor (RBAC)")

	// F — Financeiro NÃO pode criar vendedor
	_, err = svc.CreateVendor(ctx, service.CreateVendorReq{
		Name:                 "Tentativa Financeiro",
		Email:                "f9a-financeiro-tentativa@test.com",
		CommissionPercentage: decimal.NewFromFloat(5.0),
	}, "financeiro")
	require.Error(t, err, "financeiro não pode criar vendedor (RBAC)")

	// G — Vendedor NÃO pode anonimizar (LGPD)
	err = svc.AnonymizeVendor(ctx, vendor2ID, user1ID, "vendedor")
	require.Error(t, err, "vendedor não pode anonimizar outro vendedor")

	// H — Financeiro NÃO pode anonimizar
	err = svc.AnonymizeVendor(ctx, vendor2ID, gestorID, "financeiro")
	require.Error(t, err, "financeiro não pode anonimizar (apenas gestor)")

	// I — Comissões: escopo isolado por vendor_id (SC-005)
	_, err = ruleRepo.CreateRule(ctx, vendor1ID, decimal.NewFromFloat(8.0))
	require.NoError(t, err)

	orderRepo := repository.NewPGOrderRepository(pool)
	// Usar setembro (mês futuro vs. CURRENT_DATE=junho 2026) para garantir que a regra está ativa
	order, err := orderRepo.Create(ctx, repository.CreateOrderReq{
		VendorID:   vendor1ID,
		TotalCents: 50000,
		OrderDate:  time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	_, err = orderRepo.Transition(ctx, order.ID, domain.OrderStatusConfirmado, gestorID)
	require.NoError(t, err)
	_, err = orderRepo.Transition(ctx, order.ID, domain.OrderStatusPago, gestorID)
	require.NoError(t, err)

	commRepo := repository.NewPGCommissionRepository(pool)
	apurSvc := service.NewApurationService(orderRepo, ruleRepo, commRepo)
	_, err = apurSvc.ApurateMonth(ctx, 2026, 9, "gestor")
	require.NoError(t, err)

	// vendor2 busca comissões filtradas pelo seu ID → deve ver 0 (não vaza dados de vendor1)
	commsOfV2, err := commRepo.FindByPeriod(ctx, repository.CommissionFilter{
		VendorID:    strPtr(vendor2ID),
		PeriodYear:  intPtr(2026),
		PeriodMonth: intPtr(9),
	})
	require.NoError(t, err)
	assert.Empty(t, commsOfV2, "vendor2 não deve ter comissões (SC-005: escopo isolado)")

	// vendor1 tem suas comissões
	commsOfV1, err := commRepo.FindByPeriod(ctx, repository.CommissionFilter{
		VendorID:    strPtr(vendor1ID),
		PeriodYear:  intPtr(2026),
		PeriodMonth: intPtr(9),
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(commsOfV1), 1, "vendor1 deve ter comissões apuradas")
}

// ─── 9.1.4 — Schema float: SC-007 ────────────────────────────────────────────

// TestFase9_SchemaFloatZeroColunas (9.1.4): query information_schema.columns → 0 colunas float
// em campos monetários (SC-007/CHK063 — Constitution P-III NON-NEGOTIABLE).
func TestFase9_SchemaFloatZeroColunas(t *testing.T) {
	pool := fase9Pool(t)
	ctx := context.Background()

	// Query canônica SC-007: nenhuma coluna pode ser float
	rows, err := pool.Query(ctx, `
		SELECT table_name, column_name, data_type
		FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND data_type IN ('real', 'double precision', 'float4', 'float8')
		ORDER BY table_name, column_name
	`)
	require.NoError(t, err)
	defer rows.Close()

	type floatCol struct{ Table, Column, DataType string }
	var violations []floatCol
	for rows.Next() {
		var c floatCol
		require.NoError(t, rows.Scan(&c.Table, &c.Column, &c.DataType))
		violations = append(violations, c)
	}
	require.NoError(t, rows.Err())
	assert.Empty(t, violations,
		"SC-007 VIOLAÇÃO — P-III NON-NEGOTIABLE: nenhuma coluna pode ser float. Violações: %+v", violations)

	// Verificar que colunas _cents são BIGINT em tabelas base
	rows2, err := pool.Query(ctx, `
		SELECT c.table_name, c.column_name, c.data_type
		FROM information_schema.columns c
		JOIN information_schema.tables t
		  ON t.table_schema = c.table_schema AND t.table_name = c.table_name
		WHERE c.table_schema = 'public'
		  AND t.table_type = 'BASE TABLE'
		  AND c.column_name LIKE '%_cents'
		  AND c.data_type != 'bigint'
		ORDER BY c.table_name, c.column_name
	`)
	require.NoError(t, err)
	defer rows2.Close()

	var centsViolations []floatCol
	for rows2.Next() {
		var c floatCol
		require.NoError(t, rows2.Scan(&c.Table, &c.Column, &c.DataType))
		centsViolations = append(centsViolations, c)
	}
	require.NoError(t, rows2.Err())
	assert.Empty(t, centsViolations,
		"SC-007: todas as colunas '_cents' devem ser BIGINT. Violações: %+v", centsViolations)
}

// ─── 9.1.5 — audit_trail: toda transição registra ator + timestamp + estados ──

// TestFase9_AuditTrailTransicoes (9.1.5): toda transição de estado registra ator,
// timestamp, from_state, to_state em audit_trail (SC-006).
func TestFase9_AuditTrailTransicoes(t *testing.T) {
	pool := fase9Pool(t)
	ctx := context.Background()

	const (
		vendorID = "f9a50001-0000-0000-0000-000000000001"
		actorID  = "f9a50001-0000-0000-0000-000000000002"
	)

	t.Cleanup(func() {
		execBypass(t, pool, `DELETE FROM audit_trail WHERE entity_id IN (SELECT id FROM orders WHERE vendor_id = $1)`, vendorID)
		execBypass(t, pool, `DELETE FROM order_items WHERE order_id IN (SELECT id FROM orders WHERE vendor_id = $1)`, vendorID)
		execBypass(t, pool, `DELETE FROM orders WHERE vendor_id = $1`, vendorID)
		execBypass(t, pool, `DELETE FROM vendors WHERE id = $1`, vendorID)
		execBypass(t, pool, `DELETE FROM users WHERE id = $1`, actorID)
	})

	insertVendor(t, pool, vendorID, "Vendedor AuditTrail", "f9a-audit@test.com")
	insertUser(t, pool, actorID, "Gestor AuditTrail", "f9a-audit-gestor@test.com", "gestor", "")

	orderRepo := repository.NewPGOrderRepository(pool)

	order, err := orderRepo.Create(ctx, repository.CreateOrderReq{
		VendorID:   vendorID,
		TotalCents: 75000,
		OrderDate:  time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	before := time.Now().UTC().Add(-2 * time.Second)
	_, err = orderRepo.Transition(ctx, order.ID, domain.OrderStatusConfirmado, actorID)
	require.NoError(t, err)
	_, err = orderRepo.Transition(ctx, order.ID, domain.OrderStatusCancelado, actorID)
	require.NoError(t, err)

	// Verificar audit_trail
	type auditRow struct {
		ActorUserID string
		OccurredAt  time.Time
		FromState   string
		ToState     string
	}
	rows, err := pool.Query(ctx, `
		SELECT actor_user_id::text, occurred_at, from_state, to_state
		FROM audit_trail
		WHERE entity_type = 'order' AND entity_id = $1
		ORDER BY occurred_at ASC
	`, order.ID)
	require.NoError(t, err)
	defer rows.Close()

	var audits []auditRow
	for rows.Next() {
		var a auditRow
		require.NoError(t, rows.Scan(&a.ActorUserID, &a.OccurredAt, &a.FromState, &a.ToState))
		audits = append(audits, a)
	}
	require.NoError(t, rows.Err())

	require.Len(t, audits, 2, "devem haver 2 registros de audit_trail")

	// Registro 1: rascunho→confirmado
	assert.Equal(t, actorID, audits[0].ActorUserID, "ator deve ser gravado corretamente")
	assert.True(t, audits[0].OccurredAt.After(before),
		"occurred_at deve ser timestamp recente (after %v, got %v)", before, audits[0].OccurredAt)
	assert.Equal(t, "rascunho", audits[0].FromState)
	assert.Equal(t, "confirmado", audits[0].ToState)

	// Registro 2: confirmado→cancelado
	assert.Equal(t, actorID, audits[1].ActorUserID)
	assert.Equal(t, "confirmado", audits[1].FromState)
	assert.Equal(t, "cancelado", audits[1].ToState)

	// Verificar imutabilidade: UPDATE em audit_trail deve ser bloqueado pelo trigger
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	_, updateErr := conn.Exec(ctx,
		`UPDATE audit_trail SET reason = 'tentativa de adulteração' WHERE entity_id = $1`,
		order.ID)
	assert.Error(t, updateErr,
		"UPDATE em audit_trail deve ser bloqueado pelo trigger fn_prevent_mutation (P-I)")
}

// ─── 9.1.6 — LGPD Anonimização ───────────────────────────────────────────────

// TestFase9_LGPD_Anonimizacao (9.1.6): anonimizar vendedor → PII substituída +
// audit_trail gravada (CHK078) + registros financeiros preservados.
func TestFase9_LGPD_Anonimizacao(t *testing.T) {
	pool := fase9Pool(t)
	ctx := context.Background()

	const (
		vendorID = "f9a60001-0000-0000-0000-000000000001"
		gestorID = "f9a60001-0000-0000-0000-000000000002"
	)
	originalName := "João Silva LGPD Test"
	originalEmail := fmt.Sprintf("joao-lgpd-%s@test.com", vendorID[:8])

	t.Cleanup(func() {
		execBypass(t, pool, `DELETE FROM audit_trail WHERE entity_id = $1`, vendorID)
		execBypass(t, pool, `DELETE FROM commissions WHERE vendor_id = $1`, vendorID)
		execBypass(t, pool, `DELETE FROM audit_trail WHERE entity_id IN (SELECT id FROM orders WHERE vendor_id = $1)`, vendorID)
		execBypass(t, pool, `DELETE FROM order_items WHERE order_id IN (SELECT id FROM orders WHERE vendor_id = $1)`, vendorID)
		execBypass(t, pool, `DELETE FROM orders WHERE vendor_id = $1`, vendorID)
		execBypass(t, pool, `DELETE FROM commission_rules WHERE vendor_id = $1`, vendorID)
		execBypass(t, pool, `DELETE FROM vendors WHERE id = $1`, vendorID)
		execBypass(t, pool, `DELETE FROM users WHERE id = $1`, gestorID)
	})

	insertUser(t, pool, gestorID, "Gestor LGPD", "f9a-lgpd-gestor@test.com", "gestor", "")
	insertVendor(t, pool, vendorID, originalName, originalEmail)

	// Criar dados financeiros associados ao vendedor
	ruleRepo := repository.NewPGCommissionRuleRepository(pool)
	rule, err := ruleRepo.CreateRule(ctx, vendorID, decimal.NewFromFloat(7.5))
	require.NoError(t, err)

	orderRepo := repository.NewPGOrderRepository(pool)
	// Usar outubro (mês futuro vs. CURRENT_DATE=junho 2026) para garantir que a regra está ativa
	order, err := orderRepo.Create(ctx, repository.CreateOrderReq{
		VendorID:   vendorID,
		TotalCents: 200000,
		OrderDate:  time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	_, err = orderRepo.Transition(ctx, order.ID, domain.OrderStatusConfirmado, gestorID)
	require.NoError(t, err)
	_, err = orderRepo.Transition(ctx, order.ID, domain.OrderStatusPago, gestorID)
	require.NoError(t, err)

	commRepo := repository.NewPGCommissionRepository(pool)
	apurSvc := service.NewApurationService(orderRepo, ruleRepo, commRepo)
	_, err = apurSvc.ApurateMonth(ctx, 2026, 10, "gestor")
	require.NoError(t, err)

	// Anonimizar via service (apenas Gestor)
	vendorRepo := repository.NewPGVendorRepository(pool)
	vendorSvc := service.NewVendorService(vendorRepo, ruleRepo, &nullBlocklist{})
	err = vendorSvc.AnonymizeVendor(ctx, vendorID, gestorID, "gestor")
	require.NoError(t, err, "anonimizar vendedor como gestor deve funcionar")

	// 1. PII substituída
	v, err := vendorRepo.FindByID(ctx, vendorID)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(v.Name, "REMOVED_"),
		"name deve ser 'REMOVED_<uuid>', got: %q", v.Name)
	assert.True(t, strings.HasSuffix(v.Email, "@anon.invalid"),
		"email deve ter sufixo '@anon.invalid', got: %q", v.Email)
	assert.NotEqual(t, originalName, v.Name, "PII name não deve mais existir")
	assert.NotEqual(t, originalEmail, v.Email, "PII email não deve mais existir")
	assert.NotNil(t, v.AnonymizedAt, "anonymized_at deve estar preenchido")

	// 2. audit_trail com entity_type='vendor_anonymization' (CHK078)
	var auditCount int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_trail WHERE entity_type = 'vendor_anonymization' AND entity_id = $1`,
		vendorID).Scan(&auditCount)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, auditCount, 1,
		"deve haver pelo menos 1 registro em audit_trail com entity_type='vendor_anonymization' (CHK078)")

	// 3. Registros financeiros preservados (CHK078)
	comms, err := commRepo.FindByPeriod(ctx, repository.CommissionFilter{
		VendorID:    strPtr(vendorID),
		PeriodYear:  intPtr(2026),
		PeriodMonth: intPtr(10),
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(comms), 1, "comissões devem ser preservadas após LGPD")
	for _, c := range comms {
		assert.Equal(t, vendorID, c.VendorID, "vendor_id nas comissões deve ser preservado")
		assert.Greater(t, c.ValueCents, int64(0), "valor das comissões deve ser preservado")
	}

	// 4. Regra de comissão preservada (rastreabilidade financeira)
	rules, err := ruleRepo.ListByVendor(ctx, vendorID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(rules), 1, "regras de comissão devem ser preservadas após LGPD")
	assert.Equal(t, rule.ID, rules[0].ID)

	// 5. Pedido preservado
	found, err := orderRepo.FindByID(ctx, order.ID)
	require.NoError(t, err)
	assert.Equal(t, vendorID, found.VendorID, "pedido deve ser preservado com vendor_id correto")
}

// ─── 9.1.2 adicional — Atomicidade com pedido cancelado (CHK081) ─────────────

// TestFase9_Apuracao_CHK081_AtomicidadePedidoCancelado (9.1.1 + CHK081):
// pedido pago → cancelado não gera comissão na apuração (FR-011: apenas pedidos 'pago').
func TestFase9_Apuracao_CHK081_AtomicidadePedidoCancelado(t *testing.T) {
	pool := fase9Pool(t)
	ctx := context.Background()

	const (
		vendorID = "f9a70001-0000-0000-0000-000000000001"
		actorID  = "f9a70001-0000-0000-0000-000000000002"
	)

	t.Cleanup(func() {
		execBypass(t, pool, `DELETE FROM commissions WHERE vendor_id = $1`, vendorID)
		execBypass(t, pool, `DELETE FROM audit_trail WHERE entity_id IN (SELECT id FROM orders WHERE vendor_id = $1)`, vendorID)
		execBypass(t, pool, `DELETE FROM order_items WHERE order_id IN (SELECT id FROM orders WHERE vendor_id = $1)`, vendorID)
		execBypass(t, pool, `DELETE FROM orders WHERE vendor_id = $1`, vendorID)
		execBypass(t, pool, `DELETE FROM commission_rules WHERE vendor_id = $1`, vendorID)
		execBypass(t, pool, `DELETE FROM vendors WHERE id = $1`, vendorID)
		execBypass(t, pool, `DELETE FROM users WHERE id = $1`, actorID)
	})

	insertVendor(t, pool, vendorID, "Vendedor Atomicidade", "f9a-atomicidade@test.com")
	insertUser(t, pool, actorID, "Gestor Atomicidade", "f9a-atomicidade-gestor@test.com", "gestor", "")

	ruleRepo := repository.NewPGCommissionRuleRepository(pool)
	_, err := ruleRepo.CreateRule(ctx, vendorID, decimal.NewFromFloat(10.0))
	require.NoError(t, err)

	orderRepo := repository.NewPGOrderRepository(pool)
	commRepo := repository.NewPGCommissionRepository(pool)
	apurSvc := service.NewApurationService(orderRepo, ruleRepo, commRepo)

	// Pedido 1: pago — deve gerar comissão
	order1, err := orderRepo.Create(ctx, repository.CreateOrderReq{
		VendorID:   vendorID,
		TotalCents: 50000, // R$ 500
		OrderDate:  time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	_, err = orderRepo.Transition(ctx, order1.ID, domain.OrderStatusConfirmado, actorID)
	require.NoError(t, err)
	_, err = orderRepo.Transition(ctx, order1.ID, domain.OrderStatusPago, actorID)
	require.NoError(t, err)

	// Pedido 2: pago depois cancelado — NÃO deve gerar comissão (FR-011)
	order2, err := orderRepo.Create(ctx, repository.CreateOrderReq{
		VendorID:   vendorID,
		TotalCents: 80000, // R$ 800
		OrderDate:  time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	_, err = orderRepo.Transition(ctx, order2.ID, domain.OrderStatusConfirmado, actorID)
	require.NoError(t, err)
	_, err = orderRepo.Transition(ctx, order2.ID, domain.OrderStatusPago, actorID)
	require.NoError(t, err)
	_, err = orderRepo.Transition(ctx, order2.ID, domain.OrderStatusCancelado, actorID)
	require.NoError(t, err)

	// Apurar julho 2026
	result, err := apurSvc.ApurateMonth(ctx, 2026, 7, "gestor")
	require.NoError(t, err)
	assert.Equal(t, 1, result.Calculated, "apenas 1 pedido pago deve gerar comissão (FR-011)")

	comms, err := commRepo.FindByPeriod(ctx, repository.CommissionFilter{
		VendorID: strPtr(vendorID),
		PeriodYear: intPtr(2026),
		PeriodMonth: intPtr(7),
	})
	require.NoError(t, err)
	assert.Len(t, comms, 1, "deve haver exatamente 1 comissão (pedido cancelado excluído)")
	if len(comms) > 0 {
		assert.Equal(t, int64(5000), comms[0].ValueCents,
			"comissão: R$500 × 10%% = R$50 = 5.000 centavos")
	}
}
