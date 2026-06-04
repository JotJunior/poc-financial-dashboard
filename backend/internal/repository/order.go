// Package repository — order repository.
// Implementa persistência de pedidos com pgx/v5.
// Constitution P-I: toda transição de estado gera registro imutável em audit_trail.
// Constitution P-III: valores sempre em centavos (int64 — zero float).
// CHK020 OWASP: todos os parâmetros usam bind params — zero interpolação SQL.
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"financial-dashboard/backend/internal/domain"
)

// ─── Erros sentinela ──────────────────────────────────────────────────────────

var (
	// ErrOrderNotFound é retornado quando o pedido não existe.
	ErrOrderNotFound = errors.New("repository: pedido não encontrado")

	// ErrOrderIDConflict é retornado quando um order_id já existe (idempotência UNIQUE).
	ErrOrderIDConflict = errors.New("repository: pedido já registrado (order_id duplicado)")
)

// ─── Domain types ─────────────────────────────────────────────────────────────

// Order representa um pedido conforme data-model.md §orders.
type Order struct {
	ID         string
	VendorID   string
	TotalCents int64
	OrderDate  time.Time
	Status     domain.OrderStatus
	PaidAt     *time.Time
	CreatedAt  time.Time
	Items      []OrderItem
}

// OrderItem representa um item de linha de pedido.
type OrderItem struct {
	ID              string
	OrderID         string
	Description     string
	Quantity        int
	UnitPriceCents  int64
	LineTotalCents  int64
	CreatedAt       time.Time
}

// CreateOrderReq contém os campos necessários para criar um pedido.
type CreateOrderReq struct {
	VendorID   string
	TotalCents int64
	OrderDate  time.Time
	Items      []CreateOrderItemReq
}

// CreateOrderItemReq contém os campos de um item de pedido para criação.
type CreateOrderItemReq struct {
	Description    string
	Quantity       int
	UnitPriceCents int64
	LineTotalCents int64
}

// OrderFilter filtra listagens de pedidos (CHK020: todos são bind params).
type OrderFilter struct {
	VendorID    *string
	Status      *domain.OrderStatus
	PeriodYear  *int
	PeriodMonth *int
}

// ─── Interface ────────────────────────────────────────────────────────────────

// OrderRepository define as operações de persistência de pedidos.
type OrderRepository interface {
	Create(ctx context.Context, req CreateOrderReq) (Order, error)
	FindByID(ctx context.Context, id string) (Order, error)
	List(ctx context.Context, filter OrderFilter) ([]Order, error)
	Transition(ctx context.Context, id string, to domain.OrderStatus, actorID string) (Order, error)
}

// ─── Implementação PostgreSQL ─────────────────────────────────────────────────

// PGOrderRepository implementa OrderRepository com pgx/v5.
type PGOrderRepository struct {
	pool *pgxpool.Pool
}

// NewPGOrderRepository cria um PGOrderRepository.
func NewPGOrderRepository(pool *pgxpool.Pool) *PGOrderRepository {
	return &PGOrderRepository{pool: pool}
}

// Create insere um novo pedido com status inicial 'rascunho'.
// Retorna ErrOrderIDConflict se o ID do pedido já existir (idempotência).
// Insere order_items em ÚNICA transação (atomicidade P-II).
func (r *PGOrderRepository) Create(ctx context.Context, req CreateOrderReq) (Order, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Order{}, fmt.Errorf("repository: iniciar tx create_order: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var order Order
	err = tx.QueryRow(ctx, `
		INSERT INTO orders (vendor_id, total_cents, order_date, status)
		VALUES ($1, $2, $3, 'rascunho')
		RETURNING id, vendor_id, total_cents, order_date, status, paid_at, created_at
	`, req.VendorID, req.TotalCents, req.OrderDate).Scan(
		&order.ID, &order.VendorID, &order.TotalCents, &order.OrderDate,
		&order.Status, &order.PaidAt, &order.CreatedAt,
	)
	if err != nil {
		if isUniqueViolationMsg(err, "orders") || isUniqueViolationMsg(err, "id") {
			return Order{}, ErrOrderIDConflict
		}
		return Order{}, fmt.Errorf("repository: inserir pedido: %w", err)
	}

	// Inserir itens de linha na mesma transação.
	for _, item := range req.Items {
		var oi OrderItem
		err = tx.QueryRow(ctx, `
			INSERT INTO order_items (order_id, description, quantity, unit_price_cents, line_total_cents)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id, order_id, description, quantity, unit_price_cents, line_total_cents, created_at
		`, order.ID, item.Description, item.Quantity, item.UnitPriceCents, item.LineTotalCents).Scan(
			&oi.ID, &oi.OrderID, &oi.Description, &oi.Quantity,
			&oi.UnitPriceCents, &oi.LineTotalCents, &oi.CreatedAt,
		)
		if err != nil {
			return Order{}, fmt.Errorf("repository: inserir item de pedido: %w", err)
		}
		order.Items = append(order.Items, oi)
	}

	if err := tx.Commit(ctx); err != nil {
		return Order{}, fmt.Errorf("repository: commit create_order: %w", err)
	}
	return order, nil
}

// FindByID retorna um pedido pelo ID com seus itens de linha (JOIN order_items).
func (r *PGOrderRepository) FindByID(ctx context.Context, id string) (Order, error) {
	var order Order
	err := r.pool.QueryRow(ctx, `
		SELECT id, vendor_id, total_cents, order_date, status, paid_at, created_at
		FROM orders
		WHERE id = $1
	`, id).Scan(
		&order.ID, &order.VendorID, &order.TotalCents, &order.OrderDate,
		&order.Status, &order.PaidAt, &order.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Order{}, ErrOrderNotFound
		}
		return Order{}, fmt.Errorf("repository: buscar pedido %s: %w", id, err)
	}

	// Carregar itens de linha.
	rows, err := r.pool.Query(ctx, `
		SELECT id, order_id, description, quantity, unit_price_cents, line_total_cents, created_at
		FROM order_items
		WHERE order_id = $1
		ORDER BY created_at
	`, id)
	if err != nil {
		return Order{}, fmt.Errorf("repository: listar itens do pedido %s: %w", id, err)
	}
	defer rows.Close()

	for rows.Next() {
		var item OrderItem
		if err := rows.Scan(
			&item.ID, &item.OrderID, &item.Description, &item.Quantity,
			&item.UnitPriceCents, &item.LineTotalCents, &item.CreatedAt,
		); err != nil {
			return Order{}, fmt.Errorf("repository: scan item de pedido: %w", err)
		}
		order.Items = append(order.Items, item)
	}
	if err := rows.Err(); err != nil {
		return Order{}, fmt.Errorf("repository: itens do pedido (rows): %w", err)
	}

	return order, nil
}

// List retorna pedidos com filtros opcionais (bind params — CHK020).
// Filtros: vendor_id, status, período (ano+mês de order_date).
func (r *PGOrderRepository) List(ctx context.Context, filter OrderFilter) ([]Order, error) {
	// Construção dinâmica de WHERE com bind params sequenciais.
	args := []any{}
	where := []string{}
	i := 1

	if filter.VendorID != nil {
		where = append(where, fmt.Sprintf("vendor_id = $%d", i))
		args = append(args, *filter.VendorID)
		i++
	}
	if filter.Status != nil {
		where = append(where, fmt.Sprintf("status = $%d", i))
		args = append(args, string(*filter.Status))
		i++
	}
	if filter.PeriodYear != nil {
		where = append(where, fmt.Sprintf("EXTRACT(YEAR FROM order_date) = $%d", i))
		args = append(args, *filter.PeriodYear)
		i++
	}
	if filter.PeriodMonth != nil {
		where = append(where, fmt.Sprintf("EXTRACT(MONTH FROM order_date) = $%d", i))
		args = append(args, *filter.PeriodMonth)
		i++
	}

	q := `SELECT id, vendor_id, total_cents, order_date, status, paid_at, created_at FROM orders`
	if len(where) > 0 {
		q += " WHERE "
		for j, w := range where {
			if j > 0 {
				q += " AND "
			}
			q += w
		}
	}
	q += " ORDER BY order_date DESC, created_at DESC"

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("repository: listar pedidos: %w", err)
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var o Order
		if err := rows.Scan(
			&o.ID, &o.VendorID, &o.TotalCents, &o.OrderDate,
			&o.Status, &o.PaidAt, &o.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("repository: scan pedido: %w", err)
		}
		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repository: listar pedidos (rows): %w", err)
	}

	return orders, nil
}

// Transition executa uma transição de estado em ÚNICA transação:
//   1. Valida transição via domain.OrderStatus.Transition
//   2. UPDATE status (e paid_at se → pago)
//   3. INSERT em audit_trail (P-I)
//
// FR-009: atomicidade garantida por única transação pgx.
// Retorna ErrInvalidTransition se a transição não é permitida.
func (r *PGOrderRepository) Transition(ctx context.Context, id string, to domain.OrderStatus, actorID string) (Order, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Order{}, fmt.Errorf("repository: iniciar tx transition_order: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Buscar estado atual com lock de linha (NOWAIT evita deadlock silencioso).
	var current domain.OrderStatus
	err = tx.QueryRow(ctx, `
		SELECT status FROM orders WHERE id = $1 FOR UPDATE NOWAIT
	`, id).Scan(&current)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Order{}, ErrOrderNotFound
		}
		return Order{}, fmt.Errorf("repository: lock pedido %s: %w", id, err)
	}

	// Validar transição via domain (FR-008).
	result, err := current.Transition(to)
	if err != nil {
		return Order{}, err // ErrInvalidTransition já wrappado pelo domain
	}

	// Atualizar status (e paid_at se → pago).
	var order Order
	if to == domain.OrderStatusPago {
		err = tx.QueryRow(ctx, `
			UPDATE orders
			SET status = $1, paid_at = NOW()
			WHERE id = $2
			RETURNING id, vendor_id, total_cents, order_date, status, paid_at, created_at
		`, string(to), id).Scan(
			&order.ID, &order.VendorID, &order.TotalCents, &order.OrderDate,
			&order.Status, &order.PaidAt, &order.CreatedAt,
		)
	} else {
		err = tx.QueryRow(ctx, `
			UPDATE orders
			SET status = $1
			WHERE id = $2
			RETURNING id, vendor_id, total_cents, order_date, status, paid_at, created_at
		`, string(to), id).Scan(
			&order.ID, &order.VendorID, &order.TotalCents, &order.OrderDate,
			&order.Status, &order.PaidAt, &order.CreatedAt,
		)
	}
	if err != nil {
		return Order{}, fmt.Errorf("repository: atualizar status do pedido %s: %w", id, err)
	}

	// Registrar transição em audit_trail (P-I — trilha append-only).
	_, err = tx.Exec(ctx, `
		INSERT INTO audit_trail (entity_type, entity_id, actor_user_id, from_state, to_state)
		VALUES ('order', $1, $2, $3, $4)
	`, id, actorID, string(result.From), string(result.To))
	if err != nil {
		return Order{}, fmt.Errorf("repository: inserir audit_trail para pedido %s: %w", id, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Order{}, fmt.Errorf("repository: commit transition_order: %w", err)
	}
	return order, nil
}

