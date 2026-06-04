// Package repository — commission repository.
// Implementa persistência de comissões e estornos com pgx/v5.
// Constitution P-I: comissões e estornos são imutáveis — triggers bloqueiam UPDATE/DELETE.
// Constitution P-II: cálculo de comissão é responsabilidade do service; o repository
//   apenas persiste o resultado já calculado.
// Constitution P-III: valores sempre em centavos (int64 — zero float).
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"financial-dashboard/backend/internal/domain"
)

// ─── Erros sentinela ──────────────────────────────────────────────────────────

var (
	// ErrCommissionNotFound é retornado quando a comissão não existe.
	ErrCommissionNotFound = errors.New("repository: comissão não encontrada")

	// ErrCommissionAlreadyExists é retornado quando já existe comissão para o pedido.
	// A constraint UNIQUE(order_id) garante idempotência (FR-014).
	ErrCommissionAlreadyExists = errors.New("repository: comissão já calculada para este pedido")
)

// ─── Domain types ─────────────────────────────────────────────────────────────

// Commission representa uma comissão calculada conforme data-model.md §commissions.
type Commission struct {
	ID                string
	OrderID           string
	VendorID          string
	ValueCents        int64
	AppliedPercentage decimal.Decimal
	RuleID            string
	PeriodYear        int
	PeriodMonth       int
	Status            domain.CommissionStatus
	CalculatedAt      time.Time
}

// CommissionReversal representa um estorno de comissão (FR-027/FR-028).
type CommissionReversal struct {
	ID           string
	CommissionID string
	OrderID      string
	ValueCents   int64 // sempre <= 0 (constraint no banco)
	Status       domain.ReversalStatus
	CreatedAt    time.Time
	ActorUserID  string
}

// CreateCommissionReq contém os campos para criar uma comissão.
type CreateCommissionReq struct {
	OrderID           string
	VendorID          string
	ValueCents        int64
	AppliedPercentage decimal.Decimal
	RuleID            string
	PeriodYear        int
	PeriodMonth       int
}

// CreateReversalReq contém os campos para criar um estorno de comissão.
type CreateReversalReq struct {
	CommissionID string
	OrderID      string
	ValueCents   int64 // deve ser <= 0
	Status       domain.ReversalStatus
	ActorUserID  string
}

// ─── Interface ────────────────────────────────────────────────────────────────

// CommissionRepository define as operações de persistência de comissões.
type CommissionRepository interface {
	Create(ctx context.Context, req CreateCommissionReq) (Commission, error)
	FindByOrderID(ctx context.Context, orderID string) (Commission, error)
	ListByVendorPeriod(ctx context.Context, vendorID string, year, month int) ([]Commission, error)
	CreateReversal(ctx context.Context, tx pgx.Tx, req CreateReversalReq) (CommissionReversal, error)
}

// ─── Implementação PostgreSQL ─────────────────────────────────────────────────

// PGCommissionRepository implementa CommissionRepository com pgx/v5.
type PGCommissionRepository struct {
	pool *pgxpool.Pool
}

// NewPGCommissionRepository cria um PGCommissionRepository.
func NewPGCommissionRepository(pool *pgxpool.Pool) *PGCommissionRepository {
	return &PGCommissionRepository{pool: pool}
}

// Create insere uma nova comissão.
// UNIQUE(order_id) garante idempotência (FR-014) — retorna ErrCommissionAlreadyExists se duplicado.
func (r *PGCommissionRepository) Create(ctx context.Context, req CreateCommissionReq) (Commission, error) {
	var c Commission
	err := r.pool.QueryRow(ctx, `
		INSERT INTO commissions
		  (order_id, vendor_id, value_cents, applied_percentage, rule_id,
		   period_year, period_month, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'pendente')
		RETURNING id, order_id, vendor_id, value_cents, applied_percentage, rule_id,
		          period_year, period_month, status, calculated_at
	`,
		req.OrderID, req.VendorID, req.ValueCents, req.AppliedPercentage.String(),
		req.RuleID, req.PeriodYear, req.PeriodMonth,
	).Scan(
		&c.ID, &c.OrderID, &c.VendorID, &c.ValueCents, &c.AppliedPercentage,
		&c.RuleID, &c.PeriodYear, &c.PeriodMonth, &c.Status, &c.CalculatedAt,
	)
	if err != nil {
		if isUniqueViolation(err, "commissions_order_id_key") || isUniqueViolationMsg(err, "order_id") {
			return Commission{}, ErrCommissionAlreadyExists
		}
		return Commission{}, fmt.Errorf("repository: inserir comissão: %w", err)
	}
	return c, nil
}

// FindByOrderID retorna a comissão de um pedido.
func (r *PGCommissionRepository) FindByOrderID(ctx context.Context, orderID string) (Commission, error) {
	var c Commission
	err := r.pool.QueryRow(ctx, `
		SELECT id, order_id, vendor_id, value_cents, applied_percentage, rule_id,
		       period_year, period_month, status, calculated_at
		FROM commissions
		WHERE order_id = $1
	`, orderID).Scan(
		&c.ID, &c.OrderID, &c.VendorID, &c.ValueCents, &c.AppliedPercentage,
		&c.RuleID, &c.PeriodYear, &c.PeriodMonth, &c.Status, &c.CalculatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Commission{}, ErrCommissionNotFound
		}
		return Commission{}, fmt.Errorf("repository: buscar comissão do pedido %s: %w", orderID, err)
	}
	return c, nil
}

// ListByVendorPeriod retorna comissões de um vendedor num período (ano+mês).
func (r *PGCommissionRepository) ListByVendorPeriod(ctx context.Context, vendorID string, year, month int) ([]Commission, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, order_id, vendor_id, value_cents, applied_percentage, rule_id,
		       period_year, period_month, status, calculated_at
		FROM commissions
		WHERE vendor_id = $1 AND period_year = $2 AND period_month = $3
		ORDER BY calculated_at DESC
	`, vendorID, year, month)
	if err != nil {
		return nil, fmt.Errorf("repository: listar comissões do vendedor %s em %d/%02d: %w", vendorID, year, month, err)
	}
	defer rows.Close()

	var out []Commission
	for rows.Next() {
		var c Commission
		if err := rows.Scan(
			&c.ID, &c.OrderID, &c.VendorID, &c.ValueCents, &c.AppliedPercentage,
			&c.RuleID, &c.PeriodYear, &c.PeriodMonth, &c.Status, &c.CalculatedAt,
		); err != nil {
			return nil, fmt.Errorf("repository: scan comissão: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repository: listar comissões (rows): %w", err)
	}
	return out, nil
}

// CreateReversal insere um estorno de comissão usando uma transação existente.
// Recebe pgx.Tx para operar dentro da mesma transação de cancelamento do pedido
// (atomicidade CHK081 — P-II).
// value_cents DEVE ser <= 0 (constraint no banco).
func (r *PGCommissionRepository) CreateReversal(ctx context.Context, tx pgx.Tx, req CreateReversalReq) (CommissionReversal, error) {
	var rev CommissionReversal
	err := tx.QueryRow(ctx, `
		INSERT INTO commission_reversals
		  (commission_id, order_id, value_cents, status, actor_user_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, commission_id, order_id, value_cents, status, created_at, actor_user_id
	`,
		req.CommissionID, req.OrderID, req.ValueCents, string(req.Status), req.ActorUserID,
	).Scan(
		&rev.ID, &rev.CommissionID, &rev.OrderID, &rev.ValueCents,
		&rev.Status, &rev.CreatedAt, &rev.ActorUserID,
	)
	if err != nil {
		return CommissionReversal{}, fmt.Errorf("repository: inserir estorno: %w", err)
	}
	return rev, nil
}

// BeginTx inicia uma transação para operações que precisam de atomicidade cross-repo.
// Usado pelo OrderService para a operação de cancelamento com estorno (CHK081).
func (r *PGCommissionRepository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.pool.Begin(ctx)
}
