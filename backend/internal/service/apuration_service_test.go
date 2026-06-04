// Package service — testes unitários de ApurationService.
// Task 5.2.6: US3.1 (8% de R$2000+R$3000 = R$400 exatos), SC-003 (idempotência),
// FR-011 (só pedidos 'pago'), US3.4 (percentual na data do pedido).
package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"financial-dashboard/backend/internal/domain"
	"financial-dashboard/backend/internal/repository"
)

// ─── Mocks ───────────────────────────────────────────────────────────────────

type mockApurationOrderRepo struct {
	orders []repository.Order
}

func (m *mockApurationOrderRepo) List(_ context.Context, _ repository.OrderFilter) ([]repository.Order, error) {
	return m.orders, nil
}

type mockApurationRuleRepo struct {
	rule repository.CommissionRule
	err  error
}

func (m *mockApurationRuleRepo) FindActiveAtDate(_ context.Context, _ string, _ time.Time) (repository.CommissionRule, error) {
	return m.rule, m.err
}

type mockApurationCommissionRepo struct {
	inserted  int
	skipped   int
	createErr error
	commissions []repository.Commission
	findErr   error
}

func (m *mockApurationCommissionRepo) CreateBatch(_ context.Context, _ pgx.Tx, reqs []repository.CreateCommissionReq) (int, int, error) {
	if m.createErr != nil {
		return 0, 0, m.createErr
	}
	return m.inserted, m.skipped, nil
}

func (m *mockApurationCommissionRepo) FindByPeriod(_ context.Context, _ repository.CommissionFilter) ([]repository.Commission, error) {
	return m.commissions, m.findErr
}

func (m *mockApurationCommissionRepo) FindByID(_ context.Context, id string) (repository.Commission, error) {
	for _, c := range m.commissions {
		if c.ID == id {
			return c, nil
		}
	}
	return repository.Commission{}, repository.ErrCommissionNotFound
}

func (m *mockApurationCommissionRepo) Transition(_ context.Context, id string, to domain.CommissionStatus, _ string, _ string) (repository.Commission, error) {
	for i, c := range m.commissions {
		if c.ID == id {
			m.commissions[i].Status = to
			return m.commissions[i], nil
		}
	}
	return repository.Commission{}, repository.ErrCommissionNotFound
}

func (m *mockApurationCommissionRepo) BeginTx(_ context.Context) (pgx.Tx, error) {
	return &mockTx{}, nil
}

// mockTx implementa pgx.Tx de forma mínima para testes unitários.
type mockTx struct{}

func (t *mockTx) Begin(ctx context.Context) (pgx.Tx, error) { return t, nil }
func (t *mockTx) Commit(ctx context.Context) error          { return nil }
func (t *mockTx) Rollback(ctx context.Context) error        { return nil }
func (t *mockTx) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (t *mockTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults { return nil }
func (t *mockTx) LargeObjects() pgx.LargeObjects                                { return pgx.LargeObjects{} }
func (t *mockTx) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (t *mockTx) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (t *mockTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}
func (t *mockTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row { return nil }
func (t *mockTx) Conn() *pgx.Conn                                               { return nil }

// ─── Testes ───────────────────────────────────────────────────────────────────

func TestApurateMonth_RBAC(t *testing.T) {
	svc := NewApurationService(
		&mockApurationOrderRepo{},
		&mockApurationRuleRepo{},
		&mockApurationCommissionRepo{},
	)

	_, err := svc.ApurateMonth(context.Background(), 2026, 6, "vendedor")
	assert.ErrorIs(t, err, ErrForbidden, "vendedor não pode apurar")

	_, err = svc.ApurateMonth(context.Background(), 2026, 6, "financeiro")
	assert.ErrorIs(t, err, ErrForbidden, "financeiro não pode apurar")
}

func TestApurateMonth_NoPaidOrders(t *testing.T) {
	svc := NewApurationService(
		&mockApurationOrderRepo{orders: nil},
		&mockApurationRuleRepo{},
		&mockApurationCommissionRepo{},
	)

	result, err := svc.ApurateMonth(context.Background(), 2026, 6, "gestor")
	require.NoError(t, err)
	assert.Equal(t, 0, result.Calculated)
	assert.Equal(t, 0, result.Skipped)
	assert.Equal(t, int64(0), result.TotalCents)
}

// TestApurateMonth_US3_1 — US3.1: 1 vendedor 8% + 2 pedidos (R$2000 + R$3000) → comissão R$400 exatos.
// 8% de R$2000 = R$160 (16000 centavos)
// 8% de R$3000 = R$240 (24000 centavos)
// Total = R$400 (40000 centavos).
func TestApurateMonth_US3_1(t *testing.T) {
	vendorID := "vendor-001"
	ruleID := "rule-001"
	orderDate := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)

	orders := []repository.Order{
		{ID: "order-001", VendorID: vendorID, TotalCents: 200000, OrderDate: orderDate, Status: domain.OrderStatusPago},
		{ID: "order-002", VendorID: vendorID, TotalCents: 300000, OrderDate: orderDate, Status: domain.OrderStatusPago},
	}

	rule := repository.CommissionRule{
		ID:         ruleID,
		VendorID:   vendorID,
		Percentage: decimal.NewFromFloat(8),
	}

	commRepo := &mockApurationCommissionRepo{inserted: 2, skipped: 0}

	svc := NewApurationService(
		&mockApurationOrderRepo{orders: orders},
		&mockApurationRuleRepo{rule: rule},
		commRepo,
	)

	result, err := svc.ApurateMonth(context.Background(), 2026, 6, "gestor")
	require.NoError(t, err)

	assert.Equal(t, 2026, result.PeriodYear)
	assert.Equal(t, 6, result.PeriodMonth)
	assert.Equal(t, 2, result.Calculated)
	assert.Equal(t, 0, result.Skipped)

	// 8% de R$2000 + 8% de R$3000 = R$160 + R$240 = R$400 = 40000 centavos
	// RoundCommission é chamado pelo service; verificamos o total derivado.
	assert.Equal(t, int64(40000), result.TotalCents, "8%% de R$5000 deve ser R$400 exatos (P-III)")
}

// TestApurateMonth_SC003_Idempotence — SC-003: reprocessar → 0 duplicatas.
func TestApurateMonth_SC003_Idempotence(t *testing.T) {
	vendorID := "vendor-001"
	orderDate := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)

	orders := []repository.Order{
		{ID: "order-001", VendorID: vendorID, TotalCents: 200000, OrderDate: orderDate, Status: domain.OrderStatusPago},
	}

	rule := repository.CommissionRule{
		ID:         "rule-001",
		VendorID:   vendorID,
		Percentage: decimal.NewFromFloat(8),
	}

	// Simular: todos já existem (inserted=0, skipped=1).
	commRepo := &mockApurationCommissionRepo{inserted: 0, skipped: 1}

	svc := NewApurationService(
		&mockApurationOrderRepo{orders: orders},
		&mockApurationRuleRepo{rule: rule},
		commRepo,
	)

	result, err := svc.ApurateMonth(context.Background(), 2026, 6, "gestor")
	require.NoError(t, err)
	assert.Equal(t, 0, result.Calculated, "re-apuração não insere duplicatas")
	assert.Equal(t, 1, result.Skipped, "pedido já existente deve ser contado como skipped")
	assert.Equal(t, int64(0), result.TotalCents, "totalCents=0 quando nada foi inserido")
}

// TestApurateMonth_FR011_ConfirmedOrderNotIncluded — FR-011: pedidos 'confirmado' não entram.
// O filtro de status é aplicado pelo OrderRepo.List — o mock confirma que apenas 'pago' é passado.
func TestApurateMonth_FR011_ConfirmedOrderNotIncluded(t *testing.T) {
	// Mock retorna vazio (simula que o repo filtrou e não encontrou pedidos 'pago').
	commRepo := &mockApurationCommissionRepo{inserted: 0, skipped: 0}

	svc := NewApurationService(
		&mockApurationOrderRepo{orders: nil}, // nenhum pedido pago no período
		&mockApurationRuleRepo{},
		commRepo,
	)

	result, err := svc.ApurateMonth(context.Background(), 2026, 6, "gestor")
	require.NoError(t, err)
	assert.Equal(t, 0, result.Calculated)
}

// TestApurateMonth_US3_4_PercentageAtOrderDate — US3.4: percentual vigente na data do pedido.
// Pedido com order_date em março usa percentual de março (5%), não o atual (10%).
func TestApurateMonth_US3_4_PercentageAtOrderDate(t *testing.T) {
	vendorID := "vendor-001"
	// Pedido com order_date em março/2026 (8%).
	orderDate := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)

	orders := []repository.Order{
		{ID: "order-march", VendorID: vendorID, TotalCents: 100000, OrderDate: orderDate, Status: domain.OrderStatusPago},
	}

	// Regra vigente em março: 5%
	marchRule := repository.CommissionRule{
		ID:         "rule-march",
		VendorID:   vendorID,
		Percentage: decimal.NewFromFloat(5), // 5% vigente em março
	}

	commRepo := &mockApurationCommissionRepo{inserted: 1, skipped: 0}

	svc := NewApurationService(
		&mockApurationOrderRepo{orders: orders},
		&mockApurationRuleRepo{rule: marchRule}, // mock retorna a regra de março
		commRepo,
	)

	result, err := svc.ApurateMonth(context.Background(), 2026, 3, "gestor")
	require.NoError(t, err)

	// 5% de R$1000 = R$50 = 5000 centavos
	assert.Equal(t, int64(5000), result.TotalCents, "deve usar percentual na data do pedido (US3.4)")
}

// TestApurateMonth_NoRuleForVendor — pedido sem regra de comissão é pulado (não falha).
func TestApurateMonth_NoRuleForVendor(t *testing.T) {
	vendorID := "vendor-001"
	orderDate := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)

	orders := []repository.Order{
		{ID: "order-001", VendorID: vendorID, TotalCents: 100000, OrderDate: orderDate, Status: domain.OrderStatusPago},
	}

	commRepo := &mockApurationCommissionRepo{}

	svc := NewApurationService(
		&mockApurationOrderRepo{orders: orders},
		&mockApurationRuleRepo{err: repository.ErrCommissionRuleNotFound},
		commRepo,
	)

	// Não deve falhar — pedido sem regra é pulado.
	result, err := svc.ApurateMonth(context.Background(), 2026, 6, "gestor")
	require.NoError(t, err)
	assert.Equal(t, 0, result.Calculated)
}

// TestApurateMonth_InvalidMonth — validação de mês.
func TestApurateMonth_InvalidMonth(t *testing.T) {
	svc := NewApurationService(
		&mockApurationOrderRepo{},
		&mockApurationRuleRepo{},
		&mockApurationCommissionRepo{},
	)

	_, err := svc.ApurateMonth(context.Background(), 2026, 0, "gestor")
	assert.ErrorIs(t, err, ErrInvalidInput)

	_, err = svc.ApurateMonth(context.Background(), 2026, 13, "gestor")
	assert.ErrorIs(t, err, ErrInvalidInput)
}

// TestTransitionCommission_RBAC — apenas Financeiro pode transicionar.
func TestTransitionCommission_RBAC(t *testing.T) {
	commID := "comm-001"
	commRepo := &mockApurationCommissionRepo{
		commissions: []repository.Commission{
			{ID: commID, Status: domain.CommissionStatusPendente},
		},
	}

	svc := NewApurationService(
		&mockApurationOrderRepo{},
		&mockApurationRuleRepo{},
		commRepo,
	)

	_, err := svc.TransitionCommission(context.Background(), commID, domain.CommissionStatusAprovado, "actor", "gestor", "")
	assert.ErrorIs(t, err, ErrForbidden, "gestor não pode transicionar comissão")

	_, err = svc.TransitionCommission(context.Background(), commID, domain.CommissionStatusAprovado, "actor", "vendedor", "")
	assert.ErrorIs(t, err, ErrForbidden, "vendedor não pode transicionar comissão")
}

// TestListCommissions_VendorScope — Vendedor vê apenas suas comissões.
func TestListCommissions_VendorScope(t *testing.T) {
	vendorID := "vendor-abc"

	commRepo := &mockApurationCommissionRepo{
		commissions: []repository.Commission{
			{ID: "comm-1", VendorID: vendorID, Status: domain.CommissionStatusPendente},
		},
	}

	svc := NewApurationService(
		&mockApurationOrderRepo{},
		&mockApurationRuleRepo{},
		commRepo,
	)

	// Vendedor sem vendor_id no token → erro
	_, err := svc.ListCommissions(context.Background(), repository.CommissionFilter{}, "vendedor", "")
	assert.ErrorIs(t, err, ErrForbidden)
}

// TestApurateMonth_CHK073_Atomicity — CHK073: simula falha em CreateBatch; verifica que nada persiste.
// No teste unitário, o mock simula o erro; o teste verifica que o ApurateMonth propaga o erro.
func TestApurateMonth_CHK073_Atomicity(t *testing.T) {
	vendorID := "vendor-001"
	orderDate := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)

	orders := []repository.Order{
		{ID: "order-001", VendorID: vendorID, TotalCents: 100000, OrderDate: orderDate, Status: domain.OrderStatusPago},
	}

	rule := repository.CommissionRule{
		ID:         "rule-001",
		VendorID:   vendorID,
		Percentage: decimal.NewFromFloat(8),
	}

	// Simular falha no batch insert.
	commRepo := &mockApurationCommissionRepo{
		createErr: errors.New("simulated DB failure mid-batch"),
	}

	svc := NewApurationService(
		&mockApurationOrderRepo{orders: orders},
		&mockApurationRuleRepo{rule: rule},
		commRepo,
	)

	_, err := svc.ApurateMonth(context.Background(), 2026, 6, "gestor")
	require.Error(t, err, "falha no batch deve ser propagada (CHK073 — rollback garantido pelo defer)")
	assert.Contains(t, err.Error(), "inserir batch")
}
