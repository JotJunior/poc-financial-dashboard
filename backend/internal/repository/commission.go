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
	"strings"
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
	NetCents          int64 // valor líquido (bruto + estornos); preenchido pela view commission_net_balance
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

// CommissionFilter filtra listagens de comissões (CHK020: todos são bind params).
type CommissionFilter struct {
	VendorID    *string
	PeriodYear  *int
	PeriodMonth *int
	Status      *domain.CommissionStatus
}

// CommissionNetBalance contém os dados de uma comissão com saldo líquido
// derivado da view commission_net_balance (data-model.md §009_views.sql).
type CommissionNetBalance struct {
	CommissionID string
	VendorID     string
	NetCents     int64 // value_cents + SUM(reversals.value_cents) — pode ser < bruto se houver estornos
}

// ─── Interface ────────────────────────────────────────────────────────────────

// CommissionRepository define as operações de persistência de comissões.
type CommissionRepository interface {
	Create(ctx context.Context, req CreateCommissionReq) (Commission, error)
	FindByID(ctx context.Context, id string) (Commission, error)
	FindByOrderID(ctx context.Context, orderID string) (Commission, error)
	ListByVendorPeriod(ctx context.Context, vendorID string, year, month int) ([]Commission, error)
	CreateReversal(ctx context.Context, tx pgx.Tx, req CreateReversalReq) (CommissionReversal, error)
	BeginTx(ctx context.Context) (pgx.Tx, error)

	// 5.1 — Apuração de comissões (FASE 5)
	CreateBatch(ctx context.Context, tx pgx.Tx, reqs []CreateCommissionReq) (inserted int, skipped int, err error)
	FindByPeriod(ctx context.Context, filter CommissionFilter) ([]Commission, error)
	Transition(ctx context.Context, id string, to domain.CommissionStatus, actorID string, motivo string) (Commission, error)
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
	c.NetCents = c.ValueCents // sem estornos ainda
	return c, nil
}

// FindByID retorna uma comissão pelo seu ID com saldo líquido da view.
func (r *PGCommissionRepository) FindByID(ctx context.Context, id string) (Commission, error) {
	var c Commission
	err := r.pool.QueryRow(ctx, `
		SELECT c.id, c.order_id, c.vendor_id, c.value_cents,
		       COALESCE(nb.net_cents, c.value_cents) AS net_cents,
		       c.applied_percentage, c.rule_id,
		       c.period_year, c.period_month, c.status, c.calculated_at
		FROM commissions c
		LEFT JOIN commission_net_balance nb ON nb.commission_id = c.id
		WHERE c.id = $1
	`, id).Scan(
		&c.ID, &c.OrderID, &c.VendorID, &c.ValueCents, &c.NetCents,
		&c.AppliedPercentage, &c.RuleID,
		&c.PeriodYear, &c.PeriodMonth, &c.Status, &c.CalculatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Commission{}, ErrCommissionNotFound
		}
		return Commission{}, fmt.Errorf("repository: buscar comissão %s: %w", id, err)
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
	c.NetCents = c.ValueCents
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
		c.NetCents = c.ValueCents
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

// ─── 5.1 — Apuração (FASE 5) ──────────────────────────────────────────────────

// CreateBatch insere comissões em batch usando ON CONFLICT (order_id) DO NOTHING.
// Retorna (inserted, skipped, err):
//   - inserted: número de comissões efetivamente inseridas
//   - skipped: número de comissões puladas por idempotência (FR-014, SC-003)
//
// DEVE ser chamado dentro de uma transação (tx) para garantir atomicidade (CHK073).
func (r *PGCommissionRepository) CreateBatch(ctx context.Context, tx pgx.Tx, reqs []CreateCommissionReq) (inserted int, skipped int, err error) {
	if len(reqs) == 0 {
		return 0, 0, nil
	}

	// Construir INSERT batch com VALUES múltiplos.
	// ON CONFLICT (order_id) DO NOTHING garante idempotência (FR-014).
	var sb strings.Builder
	sb.WriteString(`
		INSERT INTO commissions
		  (order_id, vendor_id, value_cents, applied_percentage, rule_id,
		   period_year, period_month, status)
		VALUES
	`)

	args := make([]interface{}, 0, len(reqs)*7)
	for i, req := range reqs {
		if i > 0 {
			sb.WriteString(",\n")
		}
		base := i * 7
		fmt.Fprintf(&sb, "($%d, $%d, $%d, $%d, $%d, $%d, $%d, 'pendente')",
			base+1, base+2, base+3, base+4, base+5, base+6, base+7)
		args = append(args,
			req.OrderID, req.VendorID, req.ValueCents, req.AppliedPercentage.String(),
			req.RuleID, req.PeriodYear, req.PeriodMonth,
		)
	}
	sb.WriteString("\nON CONFLICT (order_id) DO NOTHING")

	tag, err := tx.Exec(ctx, sb.String(), args...)
	if err != nil {
		return 0, 0, fmt.Errorf("repository: inserir batch de comissões: %w", err)
	}

	inserted = int(tag.RowsAffected())
	skipped = len(reqs) - inserted
	return inserted, skipped, nil
}

// FindByPeriod retorna comissões com filtros opcionais, incluindo saldo líquido.
// Usa a view commission_net_balance para net_cents (data-model.md §009_views.sql).
// Todos os filtros usam bind params (CHK020 OWASP — zero interpolação SQL).
func (r *PGCommissionRepository) FindByPeriod(ctx context.Context, filter CommissionFilter) ([]Commission, error) {
	// Construção segura de WHERE com bind params (CHK020).
	where := []string{"1=1"}
	args := []interface{}{}
	idx := 1

	if filter.VendorID != nil {
		where = append(where, fmt.Sprintf("c.vendor_id = $%d", idx))
		args = append(args, *filter.VendorID)
		idx++
	}
	if filter.PeriodYear != nil {
		where = append(where, fmt.Sprintf("c.period_year = $%d", idx))
		args = append(args, *filter.PeriodYear)
		idx++
	}
	if filter.PeriodMonth != nil {
		where = append(where, fmt.Sprintf("c.period_month = $%d", idx))
		args = append(args, *filter.PeriodMonth)
		idx++
	}
	if filter.Status != nil {
		where = append(where, fmt.Sprintf("c.status = $%d", idx))
		args = append(args, string(*filter.Status))
		idx++
	}

	query := fmt.Sprintf(`
		SELECT c.id, c.order_id, c.vendor_id, c.value_cents,
		       COALESCE(nb.net_cents, c.value_cents) AS net_cents,
		       c.applied_percentage, c.rule_id,
		       c.period_year, c.period_month, c.status, c.calculated_at
		FROM commissions c
		LEFT JOIN commission_net_balance nb ON nb.commission_id = c.id
		WHERE %s
		ORDER BY c.calculated_at DESC
	`, strings.Join(where, " AND "))

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("repository: listar comissões por período: %w", err)
	}
	defer rows.Close()

	var out []Commission
	for rows.Next() {
		var c Commission
		if err := rows.Scan(
			&c.ID, &c.OrderID, &c.VendorID, &c.ValueCents, &c.NetCents,
			&c.AppliedPercentage, &c.RuleID,
			&c.PeriodYear, &c.PeriodMonth, &c.Status, &c.CalculatedAt,
		); err != nil {
			return nil, fmt.Errorf("repository: scan comissão (FindByPeriod): %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repository: listar comissões por período (rows): %w", err)
	}
	return out, nil
}

// Transition executa transição de estado de uma comissão em ÚNICA transação.
// Registra em audit_trail (P-I: auditabilidade de toda transição).
// motivo é obrigatório para aprovado→pendente (validado no domínio).
func (r *PGCommissionRepository) Transition(ctx context.Context, id string, to domain.CommissionStatus, actorID string, motivo string) (Commission, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Commission{}, fmt.Errorf("repository: iniciar tx transição de comissão: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Buscar estado atual.
	var current Commission
	err = tx.QueryRow(ctx, `
		SELECT id, order_id, vendor_id, value_cents, applied_percentage, rule_id,
		       period_year, period_month, status, calculated_at
		FROM commissions WHERE id = $1 FOR UPDATE
	`, id).Scan(
		&current.ID, &current.OrderID, &current.VendorID, &current.ValueCents,
		&current.AppliedPercentage, &current.RuleID,
		&current.PeriodYear, &current.PeriodMonth, &current.Status, &current.CalculatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Commission{}, ErrCommissionNotFound
		}
		return Commission{}, fmt.Errorf("repository: buscar comissão para transição: %w", err)
	}

	// Validar transição no domínio (puro).
	result, err := current.Status.Transition(to, motivo)
	if err != nil {
		return Commission{}, err
	}

	// Comissões são imutáveis (trigger BEFORE UPDATE → RAISE EXCEPTION).
	// Para "transicionar", inserimos em audit_trail e atualizamos apenas o campo status.
	// O trigger bloqueia UPDATE de outros campos — status é o único permitido via política.
	// Nota: o trigger em commissions bloqueia UPDATE/DELETE GLOBALMENTE.
	// A solução correta é: o campo status é a exceção explícita permitida pelo trigger
	// (ver migration 006_commissions.sql — o trigger só bloqueia mutação de campos financeiros).
	_, err = tx.Exec(ctx, `
		UPDATE commissions SET status = $1 WHERE id = $2
	`, string(to), id)
	if err != nil {
		return Commission{}, fmt.Errorf("repository: atualizar status da comissão: %w", err)
	}

	// Registrar em audit_trail (P-I — auditabilidade de toda transição de comissão).
	_, err = tx.Exec(ctx, `
		INSERT INTO audit_trail
		  (entity_type, entity_id, actor_user_id, from_state, to_state, reason)
		VALUES ('commission', $1, $2, $3, $4, $5)
	`, id, actorID, string(result.From), string(result.To), motivo)
	if err != nil {
		return Commission{}, fmt.Errorf("repository: audit_trail transição de comissão: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Commission{}, fmt.Errorf("repository: commit transição de comissão: %w", err)
	}

	current.Status = to
	current.NetCents = current.ValueCents
	return current, nil
}
