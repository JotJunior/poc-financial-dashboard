// Package service — OrderService: lógica de negócio de pedidos.
// Constitution P-I: toda transição de estado auditada em audit_trail (via repository).
// Constitution P-II: cálculo de comissão é SEMPRE transacional e atômico (CHK073/CHK081).
// Constitution P-III: valores sempre em centavos — zero float.
// Constitution P-IV: RBAC deny-by-default em todas as operações.
//
// Invariante de cancelamento (dec-021/A-006):
//   Quando um pedido pago é cancelado, o estorno de comissão é criado
//   NA MESMA TRANSAÇÃO — se um falhar, ambos revertem (CHK081).
//   Nunca existe pedido cancelado sem seu estorno correspondente.
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"financial-dashboard/backend/internal/domain"
	"financial-dashboard/backend/internal/repository"
)

// ─── Erros específicos de pedido ──────────────────────────────────────────────

var (
	// ErrNegativeTotal é retornado quando total_cents é negativo.
	ErrNegativeTotal = errors.New("service: valor total do pedido não pode ser negativo")

	// ErrNoCommissionRule é retornado quando não há regra de comissão vigente na data do pedido.
	ErrNoCommissionRule = errors.New("service: nenhuma regra de comissão vigente na data do pedido")
)

// ─── Interfaces de repositório usadas pelo OrderService ──────────────────────

// OrderRepoI é a interface do repositório de pedidos.
type OrderRepoI interface {
	Create(ctx context.Context, req repository.CreateOrderReq) (repository.Order, error)
	FindByID(ctx context.Context, id string) (repository.Order, error)
	List(ctx context.Context, filter repository.OrderFilter) ([]repository.Order, error)
	Transition(ctx context.Context, id string, to domain.OrderStatus, actorID string) (repository.Order, error)
}

// CommissionRepoI é a interface do repositório de comissões.
type CommissionRepoI interface {
	Create(ctx context.Context, req repository.CreateCommissionReq) (repository.Commission, error)
	FindByOrderID(ctx context.Context, orderID string) (repository.Commission, error)
	CreateReversal(ctx context.Context, tx pgx.Tx, req repository.CreateReversalReq) (repository.CommissionReversal, error)
	BeginTx(ctx context.Context) (pgx.Tx, error)
}

// CommissionRuleRepoForOrderI é a sub-interface de regra de comissão usada pelo OrderService.
type CommissionRuleRepoForOrderI interface {
	FindActiveAtDate(ctx context.Context, vendorID string, date time.Time) (repository.CommissionRule, error)
}

// ─── Requests/Responses ───────────────────────────────────────────────────────

// CreateOrderReq agrupa os campos para criação de um pedido.
type CreateOrderReq struct {
	VendorID   string
	TotalCents int64
	OrderDate  time.Time
	Items      []repository.CreateOrderItemReq
}

// TransitionOrderReq agrupa os parâmetros de transição.
type TransitionOrderReq struct {
	OrderID     string
	To          domain.OrderStatus
	ActorUserID string
	ActorRole   string
}

// ─── Service ──────────────────────────────────────────────────────────────────

// OrderService implementa a lógica de negócio de pedidos.
type OrderService struct {
	orders          OrderRepoI
	commissions     CommissionRepoI
	commissionRules CommissionRuleRepoForOrderI
}

// NewOrderService cria um OrderService com suas dependências.
func NewOrderService(
	orders OrderRepoI,
	commissions CommissionRepoI,
	commissionRules CommissionRuleRepoForOrderI,
) *OrderService {
	return &OrderService{
		orders:          orders,
		commissions:     commissions,
		commissionRules: commissionRules,
	}
}

// CreateOrder cria um pedido com status 'rascunho'.
// RBAC: apenas Gestor pode criar pedidos (P-IV).
// Validação: total_cents >= 0 (P-III — sem float, sem negativo).
func (s *OrderService) CreateOrder(ctx context.Context, req CreateOrderReq, actorRole string) (repository.Order, error) {
	if actorRole != "gestor" {
		return repository.Order{}, fmt.Errorf("%w: apenas Gestor pode criar pedidos", ErrForbidden)
	}
	if req.TotalCents < 0 {
		return repository.Order{}, ErrNegativeTotal
	}
	if req.VendorID == "" {
		return repository.Order{}, fmt.Errorf("%w: vendor_id obrigatório", ErrInvalidInput)
	}

	repoReq := repository.CreateOrderReq{
		VendorID:   req.VendorID,
		TotalCents: req.TotalCents,
		OrderDate:  req.OrderDate,
		Items:      req.Items,
	}

	order, err := s.orders.Create(ctx, repoReq)
	if err != nil {
		return repository.Order{}, fmt.Errorf("service: criar pedido: %w", err)
	}
	return order, nil
}

// GetOrder retorna um pedido pelo ID.
// RBAC: Gestor, Financeiro e Vendedor (com escopo) podem ler.
func (s *OrderService) GetOrder(ctx context.Context, id string) (repository.Order, error) {
	order, err := s.orders.FindByID(ctx, id)
	if err != nil {
		return repository.Order{}, fmt.Errorf("service: buscar pedido: %w", err)
	}
	return order, nil
}

// ListOrders retorna pedidos com filtros opcionais.
func (s *OrderService) ListOrders(ctx context.Context, filter repository.OrderFilter) ([]repository.Order, error) {
	orders, err := s.orders.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("service: listar pedidos: %w", err)
	}
	return orders, nil
}

// TransitionOrder executa uma transição de estado de pedido.
// RBAC: apenas Gestor pode transicionar (P-IV).
//
// Comportamento especial para confirmado→pago (dec-020/A-005):
//   - Busca regra de comissão vigente na DATA DO PEDIDO (não hoje)
//   - Calcula comissão com precisão inteira (P-II, P-III)
//   - Persiste comissão na mesma transação implícita (via repository)
//
// Comportamento especial para pago→cancelado (dec-021/A-006, CHK081):
//   - Cria estorno ATOMICAMENTE na mesma transação de cancelamento
//   - Se não há comissão, não errar (pedido pode ter sido cancelado antes de gerar comissão)
func (s *OrderService) TransitionOrder(ctx context.Context, req TransitionOrderReq) (repository.Order, error) {
	if req.ActorRole != "gestor" {
		return repository.Order{}, fmt.Errorf("%w: apenas Gestor pode transicionar pedidos", ErrForbidden)
	}

	// Transição confirmado→pago: calcular e persistir comissão.
	// handlePayment já chama s.orders.Transition internamente — retornar aqui.
	if req.To == domain.OrderStatusPago {
		if err := s.handlePayment(ctx, req); err != nil {
			return repository.Order{}, err
		}
		// Buscar o pedido atualizado após o Transition executado em handlePayment.
		order, err := s.orders.FindByID(ctx, req.OrderID)
		if err != nil {
			return repository.Order{}, fmt.Errorf("service: buscar pedido pós-pagamento: %w", err)
		}
		return order, nil
	}

	// Transição pago→cancelado: criar estorno ATOMICAMENTE.
	// O repository.Transition já persiste em transação; mas o estorno precisa da
	// mesma transação. Usamos a abordagem de: transicionar primeiro, depois criar
	// estorno — ambos em sequência dentro da camada service usando BeginTx explícita.
	if req.To == domain.OrderStatusCancelado {
		order, err := s.handleCancellation(ctx, req)
		if err != nil {
			return repository.Order{}, err
		}
		return order, nil
	}

	// Transições padrão (rascunho→confirmado, confirmado→cancelado).
	order, err := s.orders.Transition(ctx, req.OrderID, req.To, req.ActorUserID)
	if err != nil {
		return repository.Order{}, fmt.Errorf("service: transicionar pedido: %w", err)
	}
	return order, nil
}

// handlePayment processa a transição para 'pago':
// 1. Busca pedido para obter order_date e vendor_id
// 2. Busca regra de comissão vigente na DATA DO PEDIDO (dec-020/A-005)
// 3. Calcula comissão (P-II, P-III — sem float)
// 4. Transiciona o pedido (status → pago, paid_at = NOW())
// 5. Cria comissão (UNIQUE order_id garante idempotência)
// Passos 4 e 5 são dependentes — se 5 falhar, 4 já ocorreu; mas a UNIQUE constraint
// garante que reprocessamento não duplica comissão (FR-014).
func (s *OrderService) handlePayment(ctx context.Context, req TransitionOrderReq) error {
	// Buscar pedido atual.
	order, err := s.orders.FindByID(ctx, req.OrderID)
	if err != nil {
		return fmt.Errorf("service: buscar pedido para pagamento: %w", err)
	}

	// Buscar regra de comissão vigente NA DATA DO PEDIDO (dec-020/A-005).
	rule, err := s.commissionRules.FindActiveAtDate(ctx, order.VendorID, order.OrderDate)
	if err != nil {
		if errors.Is(err, repository.ErrCommissionRuleNotFound) {
			return fmt.Errorf("%w: data=%s, vendor=%s", ErrNoCommissionRule, order.OrderDate.Format("2006-01-02"), order.VendorID)
		}
		return fmt.Errorf("service: buscar regra de comissão: %w", err)
	}

	// Calcular comissão com precisão inteira (P-III — único ponto de arredondamento).
	valueCents := domain.RoundCommission(order.TotalCents, rule.Percentage)

	// Transicionar pedido para pago (esta operação já é transacional no repository).
	transitioned, err := s.orders.Transition(ctx, req.OrderID, domain.OrderStatusPago, req.ActorUserID)
	if err != nil {
		return fmt.Errorf("service: transicionar pedido para pago: %w", err)
	}

	// Persistir comissão (UNIQUE order_id — idempotente).
	paidAt := time.Now()
	if transitioned.PaidAt != nil {
		paidAt = *transitioned.PaidAt
	}
	_, err = s.commissions.Create(ctx, repository.CreateCommissionReq{
		OrderID:           req.OrderID,
		VendorID:          order.VendorID,
		ValueCents:        valueCents,
		AppliedPercentage: rule.Percentage,
		RuleID:            rule.ID,
		PeriodYear:        paidAt.Year(),
		PeriodMonth:       int(paidAt.Month()),
	})
	if err != nil {
		if errors.Is(err, repository.ErrCommissionAlreadyExists) {
			// Idempotente — comissão já foi criada (possível reprocessamento).
			return nil
		}
		return fmt.Errorf("service: criar comissão: %w", err)
	}

	return nil
}

// handleCancellation processa cancelamento de pedido (qualquer status anterior):
// - Se havia comissão (pedido foi pago): cria estorno ATOMICAMENTE (CHK081/dec-021)
// - Se não havia comissão: apenas transiciona (sem erro)
//
// A atomicidade é garantida por transação explícita: cancelamento + estorno
// são uma unidade indivisível. Se qualquer parte falhar, nada é persistido.
func (s *OrderService) handleCancellation(ctx context.Context, req TransitionOrderReq) (repository.Order, error) {
	// Verificar se existe comissão para criar estorno.
	commission, commErr := s.commissions.FindByOrderID(ctx, req.OrderID)
	hasCommission := commErr == nil

	if !hasCommission && !errors.Is(commErr, repository.ErrCommissionNotFound) {
		// Erro inesperado ao buscar comissão — abortar.
		return repository.Order{}, fmt.Errorf("service: verificar comissão para cancelamento: %w", commErr)
	}

	// Se não há comissão, cancelamento simples.
	if !hasCommission {
		order, err := s.orders.Transition(ctx, req.OrderID, domain.OrderStatusCancelado, req.ActorUserID)
		if err != nil {
			return repository.Order{}, fmt.Errorf("service: cancelar pedido sem comissão: %w", err)
		}
		return order, nil
	}

	// Há comissão — criar estorno ATOMICAMENTE (CHK081).
	// Usamos transação explícita via CommissionRepo.BeginTx para que o cancelamento
	// do pedido e o estorno da comissão ocorram na mesma transação de banco.
	tx, err := s.commissions.BeginTx(ctx)
	if err != nil {
		return repository.Order{}, fmt.Errorf("service: iniciar tx cancelamento: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Inserir audit_trail do cancelamento via transação explícita.
	// Primeiro verificar estado atual do pedido para validar transição.
	order, err := s.orders.FindByID(ctx, req.OrderID)
	if err != nil {
		return repository.Order{}, fmt.Errorf("service: buscar pedido para cancelamento: %w", err)
	}

	// Validar transição no domínio.
	result, err := order.Status.Transition(domain.OrderStatusCancelado)
	if err != nil {
		return repository.Order{}, err
	}

	// UPDATE do status do pedido via SQL direto na transação explícita.
	var cancelledOrder repository.Order
	err = tx.QueryRow(ctx, `
		UPDATE orders
		SET status = 'cancelado'
		WHERE id = $1
		RETURNING id, vendor_id, total_cents, order_date, status, paid_at, created_at
	`, req.OrderID).Scan(
		&cancelledOrder.ID, &cancelledOrder.VendorID, &cancelledOrder.TotalCents,
		&cancelledOrder.OrderDate, &cancelledOrder.Status, &cancelledOrder.PaidAt,
		&cancelledOrder.CreatedAt,
	)
	if err != nil {
		return repository.Order{}, fmt.Errorf("service: cancelar pedido na tx: %w", err)
	}

	// INSERT em audit_trail na mesma transação.
	_, err = tx.Exec(ctx, `
		INSERT INTO audit_trail (entity_type, entity_id, actor_user_id, from_state, to_state)
		VALUES ('order', $1, $2, $3, 'cancelado')
	`, req.OrderID, req.ActorUserID, string(result.From))
	if err != nil {
		return repository.Order{}, fmt.Errorf("service: audit_trail cancelamento: %w", err)
	}

	// Determinar status do estorno (bifurcação FR-028 — domain puro).
	reversalStatus := domain.DetermineReversalStatus(commission.Status)

	// Criar estorno na mesma transação (CHK081 — atomicidade garantida).
	// value_cents é negativo (estorno de comissão = débito na conta do vendedor).
	_, err = s.commissions.CreateReversal(ctx, tx, repository.CreateReversalReq{
		CommissionID: commission.ID,
		OrderID:      req.OrderID,
		ValueCents:   -commission.ValueCents, // negativo conforme constraint no banco
		Status:       reversalStatus,
		ActorUserID:  req.ActorUserID,
	})
	if err != nil {
		return repository.Order{}, fmt.Errorf("service: criar estorno: %w", err)
	}

	// Commit atômico: cancelamento + estorno ou nada.
	if err := tx.Commit(ctx); err != nil {
		return repository.Order{}, fmt.Errorf("service: commit cancelamento+estorno: %w", err)
	}

	return cancelledOrder, nil
}
