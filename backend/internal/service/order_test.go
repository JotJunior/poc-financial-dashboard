// Package service_test — testes unitários para OrderService.
// Tasks 4.2.5 (CHK081 — atomicidade) e 4.2.6 (bifurcação de estorno).
//
// Nota de design: pgx.Tx é uma interface extensa (~15 métodos) e a
// handleCancellation usa SQL direto na tx. Para manter os unitários simples
// e determinísticos, testamos:
//   - Lógica de bifurcação via domain.DetermineReversalStatus (pure function)
//   - RBAC e validação de input via stubs in-memory
//   - Caminhos de criação e transição via stubs
//
// Os testes de atomicidade (CHK081 — crash entre cancelamento e estorno)
// são cobertos em order_integration_test.go com banco real (força rollback).
package service_test

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"financial-dashboard/backend/internal/domain"
	"financial-dashboard/backend/internal/repository"
	"financial-dashboard/backend/internal/service"
)

// ─── Stubs ────────────────────────────────────────────────────────────────────

// orderStubOrderRepo simula OrderRepository em memória.
type orderStubOrderRepo struct {
	orders map[string]repository.Order
}

func newOrderStubOrderRepo() *orderStubOrderRepo {
	return &orderStubOrderRepo{orders: make(map[string]repository.Order)}
}

func (s *orderStubOrderRepo) Create(_ context.Context, req repository.CreateOrderReq) (repository.Order, error) {
	o := repository.Order{
		ID:         "order-" + strconv.Itoa(len(s.orders)+1),
		VendorID:   req.VendorID,
		TotalCents: req.TotalCents,
		OrderDate:  req.OrderDate,
		Status:     domain.OrderStatusRascunho,
		Items:      make([]repository.OrderItem, len(req.Items)),
		CreatedAt:  time.Now(),
	}
	for i, it := range req.Items {
		o.Items[i] = repository.OrderItem{
			Description:    it.Description,
			Quantity:       it.Quantity,
			UnitPriceCents: it.UnitPriceCents,
			LineTotalCents: it.LineTotalCents,
		}
	}
	s.orders[o.ID] = o
	return o, nil
}

func (s *orderStubOrderRepo) FindByID(_ context.Context, id string) (repository.Order, error) {
	o, ok := s.orders[id]
	if !ok {
		return repository.Order{}, repository.ErrOrderNotFound
	}
	return o, nil
}

func (s *orderStubOrderRepo) List(_ context.Context, _ repository.OrderFilter) ([]repository.Order, error) {
	out := make([]repository.Order, 0, len(s.orders))
	for _, o := range s.orders {
		out = append(out, o)
	}
	return out, nil
}

func (s *orderStubOrderRepo) Transition(_ context.Context, id string, to domain.OrderStatus, _ string) (repository.Order, error) {
	o, ok := s.orders[id]
	if !ok {
		return repository.Order{}, repository.ErrOrderNotFound
	}
	_, err := o.Status.Transition(to)
	if err != nil {
		return repository.Order{}, err
	}
	o.Status = to
	if to == domain.OrderStatusPago {
		now := time.Now()
		o.PaidAt = &now
	}
	s.orders[id] = o
	return o, nil
}

// orderStubRuleRepo simula CommissionRuleRepository para o OrderService.
type orderStubRuleRepo struct {
	rules map[string]repository.CommissionRule
}

func newOrderStubRuleRepo(vendorID string, pct float64) *orderStubRuleRepo {
	r := &orderStubRuleRepo{rules: make(map[string]repository.CommissionRule)}
	r.rules[vendorID] = repository.CommissionRule{
		ID:         "rule-001",
		VendorID:   vendorID,
		Percentage: decimal.NewFromFloat(pct),
		ValidFrom:  time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		Version:    1,
	}
	return r
}

func (s *orderStubRuleRepo) FindActiveAtDate(_ context.Context, vendorID string, _ time.Time) (repository.CommissionRule, error) {
	rule, ok := s.rules[vendorID]
	if !ok {
		return repository.CommissionRule{}, repository.ErrCommissionRuleNotFound
	}
	return rule, nil
}

// orderStubCommissionRepo simula CommissionRepository para o OrderService.
// Implementa apenas Create e FindByOrderID (não BeginTx nem CreateReversal via tx).
// O caminho de cancelamento com estorno é testado nos integration tests.
type orderStubCommissionRepo struct {
	commissions map[string]repository.Commission
}

func newOrderStubCommissionRepo() *orderStubCommissionRepo {
	return &orderStubCommissionRepo{commissions: make(map[string]repository.Commission)}
}

func (s *orderStubCommissionRepo) Create(_ context.Context, req repository.CreateCommissionReq) (repository.Commission, error) {
	if _, exists := s.commissions[req.OrderID]; exists {
		return repository.Commission{}, repository.ErrCommissionAlreadyExists
	}
	c := repository.Commission{
		ID:                "comm-" + strconv.Itoa(len(s.commissions)+1),
		OrderID:           req.OrderID,
		VendorID:          req.VendorID,
		ValueCents:        req.ValueCents,
		AppliedPercentage: req.AppliedPercentage,
		RuleID:            req.RuleID,
		PeriodYear:        req.PeriodYear,
		PeriodMonth:       req.PeriodMonth,
		Status:            domain.CommissionStatusPendente,
		CalculatedAt:      time.Now(),
	}
	s.commissions[req.OrderID] = c
	return c, nil
}

func (s *orderStubCommissionRepo) FindByOrderID(_ context.Context, orderID string) (repository.Commission, error) {
	c, ok := s.commissions[orderID]
	if !ok {
		return repository.Commission{}, repository.ErrCommissionNotFound
	}
	return c, nil
}

// Implementações não-utilizadas pelos testes unitários (cancelamento usa integration test).
// Satisfazem a interface CommissionRepoI mas retornam erro descritivo se chamadas.
func (s *orderStubCommissionRepo) CreateReversal(_ context.Context, _ pgx.Tx, _ repository.CreateReversalReq) (repository.CommissionReversal, error) {
	return repository.CommissionReversal{}, errors.New("stub: CreateReversal não implementado em testes unitários — use integration test")
}

func (s *orderStubCommissionRepo) BeginTx(_ context.Context) (pgx.Tx, error) {
	return nil, errors.New("stub: BeginTx não implementado em testes unitários — use integration test")
}

// ─── Testes de lógica de bifurcação de estorno (domain puro) ─────────────────

// TestDetermineReversalStatus_Pendente: comissão pendente → estorno aplicado (FR-028).
func TestDetermineReversalStatus_Pendente(t *testing.T) {
	status := domain.DetermineReversalStatus(domain.CommissionStatusPendente)
	assert.Equal(t, domain.ReversalStatusAplicado, status,
		"comissão pendente → estorno aplicado (FR-028)")
}

// TestDetermineReversalStatus_Aprovado: comissão aprovada → estorno pendente_aprovacao (FR-028).
func TestDetermineReversalStatus_Aprovado(t *testing.T) {
	status := domain.DetermineReversalStatus(domain.CommissionStatusAprovado)
	assert.Equal(t, domain.ReversalStatusPendenteAprovacao, status,
		"comissão aprovada → estorno pendente_aprovacao (FR-028)")
}

// TestDetermineReversalStatus_Pago: comissão paga → estorno pendente_aprovacao (FR-028).
func TestDetermineReversalStatus_Pago(t *testing.T) {
	status := domain.DetermineReversalStatus(domain.CommissionStatusPago)
	assert.Equal(t, domain.ReversalStatusPendenteAprovacao, status,
		"comissão paga → estorno pendente_aprovacao (FR-028)")
}

// ─── Testes de RBAC ───────────────────────────────────────────────────────────

// TestOrderService_CreateOrder_ForbiddenForVendedor: Vendedor não pode criar (P-IV).
func TestOrderService_CreateOrder_ForbiddenForVendedor(t *testing.T) {
	svc := service.NewOrderService(
		newOrderStubOrderRepo(),
		nil, // não usado neste fluxo
		newOrderStubRuleRepo("v1", 10.0),
	)
	ctx := context.Background()

	_, err := svc.CreateOrder(ctx, service.CreateOrderReq{
		VendorID:   "v1",
		TotalCents: 10000,
		OrderDate:  time.Now(),
	}, "vendedor")

	require.Error(t, err)
	assert.True(t, errors.Is(err, service.ErrForbidden),
		"vendedor não pode criar pedido (P-IV), got: %v", err)
}

// TestOrderService_TransitionOrder_ForbiddenForFinanceiro: Financeiro não pode transicionar.
func TestOrderService_TransitionOrder_ForbiddenForFinanceiro(t *testing.T) {
	orderRepo := newOrderStubOrderRepo()
	orderRepo.orders["order-x"] = repository.Order{
		ID:     "order-x",
		Status: domain.OrderStatusRascunho,
	}

	svc := service.NewOrderService(orderRepo, nil, newOrderStubRuleRepo("v1", 10.0))
	ctx := context.Background()

	_, err := svc.TransitionOrder(ctx, service.TransitionOrderReq{
		OrderID:     "order-x",
		To:          domain.OrderStatusConfirmado,
		ActorUserID: "actor",
		ActorRole:   "financeiro",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, service.ErrForbidden))
}

// ─── Testes de validação de input ─────────────────────────────────────────────

// TestOrderService_CreateOrder_NegativeTotal: total negativo é rejeitado (P-III).
func TestOrderService_CreateOrder_NegativeTotal(t *testing.T) {
	svc := service.NewOrderService(newOrderStubOrderRepo(), nil, newOrderStubRuleRepo("v1", 10.0))
	ctx := context.Background()

	_, err := svc.CreateOrder(ctx, service.CreateOrderReq{
		VendorID:   "v1",
		TotalCents: -1, // inválido
		OrderDate:  time.Now(),
	}, "gestor")

	require.Error(t, err)
	assert.True(t, errors.Is(err, service.ErrNegativeTotal),
		"total negativo deve retornar ErrNegativeTotal, got: %v", err)
}

// TestOrderService_CreateOrder_MissingVendorID: vendor_id vazio é rejeitado.
func TestOrderService_CreateOrder_MissingVendorID(t *testing.T) {
	svc := service.NewOrderService(newOrderStubOrderRepo(), nil, newOrderStubRuleRepo("v1", 10.0))
	ctx := context.Background()

	_, err := svc.CreateOrder(ctx, service.CreateOrderReq{
		VendorID:   "", // inválido
		TotalCents: 10000,
		OrderDate:  time.Now(),
	}, "gestor")

	require.Error(t, err)
	assert.True(t, errors.Is(err, service.ErrInvalidInput))
}

// ─── Testes de cálculo de comissão (P-II, P-III) ─────────────────────────────

// TestOrderService_CommissionCalculation_10pct: 10% de R$1.000 = 10000 centavos.
func TestOrderService_CommissionCalculation_10pct(t *testing.T) {
	const vendorID = "vendor-calc"
	orderRepo := newOrderStubOrderRepo()
	commRepo := newOrderStubCommissionRepo()
	ruleRepo := newOrderStubRuleRepo(vendorID, 10.0)

	svc := service.NewOrderService(orderRepo, commRepo, ruleRepo)
	ctx := context.Background()

	order, err := svc.CreateOrder(ctx, service.CreateOrderReq{
		VendorID:   vendorID,
		TotalCents: 100000, // R$ 1.000,00
		OrderDate:  time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC),
	}, "gestor")
	require.NoError(t, err)

	_, err = svc.TransitionOrder(ctx, service.TransitionOrderReq{
		OrderID: order.ID, To: domain.OrderStatusConfirmado,
		ActorUserID: "actor", ActorRole: "gestor",
	})
	require.NoError(t, err)

	_, err = svc.TransitionOrder(ctx, service.TransitionOrderReq{
		OrderID: order.ID, To: domain.OrderStatusPago,
		ActorUserID: "actor", ActorRole: "gestor",
	})
	require.NoError(t, err)

	comm, ok := commRepo.commissions[order.ID]
	require.True(t, ok, "comissão deve ser criada ao pagar")
	assert.Equal(t, int64(10000), comm.ValueCents,
		"10%% de R$1.000,00 = R$100,00 = 10000 centavos (P-III)")
	assert.Equal(t, domain.CommissionStatusPendente, comm.Status)
}

// TestOrderService_CommissionCalculation_8pct: 8% de R$5.000 = 40000 centavos.
func TestOrderService_CommissionCalculation_8pct(t *testing.T) {
	const vendorID = "vendor-8pct"
	orderRepo := newOrderStubOrderRepo()
	commRepo := newOrderStubCommissionRepo()
	ruleRepo := newOrderStubRuleRepo(vendorID, 8.0)

	svc := service.NewOrderService(orderRepo, commRepo, ruleRepo)
	ctx := context.Background()

	order, err := svc.CreateOrder(ctx, service.CreateOrderReq{
		VendorID:   vendorID,
		TotalCents: 500000, // R$ 5.000,00
		OrderDate:  time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC),
	}, "gestor")
	require.NoError(t, err)

	_, err = svc.TransitionOrder(ctx, service.TransitionOrderReq{
		OrderID: order.ID, To: domain.OrderStatusConfirmado,
		ActorUserID: "actor", ActorRole: "gestor",
	})
	require.NoError(t, err)

	_, err = svc.TransitionOrder(ctx, service.TransitionOrderReq{
		OrderID: order.ID, To: domain.OrderStatusPago,
		ActorUserID: "actor", ActorRole: "gestor",
	})
	require.NoError(t, err)

	comm, ok := commRepo.commissions[order.ID]
	require.True(t, ok)
	assert.Equal(t, int64(40000), comm.ValueCents,
		"8%% de R$5.000,00 = R$400,00 = 40000 centavos (P-III)")
}

// TestOrderService_TransitionOrder_NotFound: pedido inexistente retorna ErrOrderNotFound.
func TestOrderService_TransitionOrder_NotFound(t *testing.T) {
	svc := service.NewOrderService(newOrderStubOrderRepo(), nil, newOrderStubRuleRepo("v1", 10.0))
	ctx := context.Background()

	_, err := svc.TransitionOrder(ctx, service.TransitionOrderReq{
		OrderID:     "non-existent",
		To:          domain.OrderStatusConfirmado,
		ActorUserID: "actor",
		ActorRole:   "gestor",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, repository.ErrOrderNotFound))
}
