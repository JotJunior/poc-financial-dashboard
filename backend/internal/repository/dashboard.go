// Package repository — dashboard repository.
// Task 6.1: GetConsolidated, GetVendorDashboard, GetDrillDown.
// Constitution P-I: derivado deterministico — nenhum dado calculado aqui (views fazem).
// Constitution P-III: valores sempre em centavos (int64 — zero float).
// CHK020 owasp SQL injection: bind params em toda query — sem interpolação de string (dec-034).
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrDashboardNotFound é retornado quando não há dados para o filtro.
var ErrDashboardNotFound = errors.New("repository: nenhum dado de dashboard encontrado")

// ─── Domain types ─────────────────────────────────────────────────────────────

// DashboardFilter encapsula os filtros opcionais de período para queries de dashboard.
// Suporta filtragem por year+month (mês exato) ou por intervalo startDate..endDate.
// Se nenhum filtro for fornecido, retorna todos os dados.
type DashboardFilter struct {
	VendorID  string     // opcional — quando não vazio, filtra por vendor_id
	Year      int        // 0 = ignorar
	Month     int        // 0 = ignorar (ano inteiro)
	StartDate *time.Time // intervalo aberto; nil = ignorar
	EndDate   *time.Time // intervalo aberto; nil = ignorar
}

// VendorRanking é o item do ranking de vendedores por volume de vendas.
type VendorRanking struct {
	VendorID    string
	VendorName  string
	TotalCents  int64 // total de pedidos pagos em centavos
	OrderCount  int
	CommCents   int64 // total de comissões (net) em centavos
}

// ConsolidatedDashboard agrega métricas de todos os vendedores (visão Gestor/Financeiro).
type ConsolidatedDashboard struct {
	TotalSalesCents      int64 // SUM de pedidos pagos
	TotalCommCents       int64 // SUM de comissões net (via view)
	PendingCommCents     int64 // SUM de comissões status=pendente
	ApprovedCommCents    int64 // SUM de comissões status=aprovado (prontas p/ pagar)
	PaidCommCents        int64 // SUM de comissões status=pago
	OrderCount           int
	VendorCount          int
	TopVendors           []VendorRanking
}

// VendorDashboard agrega métricas de um único vendedor (visão Vendedor ou Gestor filtrado).
type VendorDashboard struct {
	VendorID          string
	TotalSalesCents   int64
	TotalCommCents    int64 // net (via view)
	PendingCommCents  int64
	ApprovedCommCents int64
	PaidCommCents     int64
	OrderCount        int
}

// DrillDown carrega um pedido + suas comissões + estornos para rastreabilidade SC-004.
type DrillDown struct {
	OrderID     string
	VendorID    string
	TotalCents  int64
	OrderDate   time.Time
	Status      string
	CreatedAt   time.Time
	Commissions []DrillDownCommission
}

// DrillDownCommission é a comissão dentro de um DrillDown.
type DrillDownCommission struct {
	CommissionID  string
	ValueCents    int64
	NetCents      int64 // após estornos
	Status        string
	PeriodYear    int
	PeriodMonth   int
	Reversals     []DrillDownReversal
}

// DrillDownReversal é um estorno dentro de uma comissão no DrillDown.
type DrillDownReversal struct {
	ReversalID  string
	ValueCents  int64 // negativo
	Status      string
	CreatedAt   time.Time
	ActorUserID string // usuário que registrou o estorno (auditabilidade P-I)
}

// ─── Interface ────────────────────────────────────────────────────────────────

// DashboardRepository define as operações de leitura para dashboard.
// Task 6.1.1
type DashboardRepository interface {
	GetConsolidated(ctx context.Context, filter DashboardFilter) (ConsolidatedDashboard, error)
	GetVendorDashboard(ctx context.Context, vendorID string, filter DashboardFilter) (VendorDashboard, error)
	GetDrillDown(ctx context.Context, orderID string) (DrillDown, error)
	GetPendingCommissionsSummary(ctx context.Context) (PendingCommissionsSummary, error)
}

// PendingCommissionsSummary agrega pendências de comissão (FR-019).
type PendingCommissionsSummary struct {
	PendingApprovalCount int
	PendingApprovalCents int64
	ApprovedUnpaidCount  int
	ApprovedUnpaidCents  int64
}

// ─── Implementation ───────────────────────────────────────────────────────────

// PGDashboardRepository implementa DashboardRepository sobre PostgreSQL.
type PGDashboardRepository struct {
	pool *pgxpool.Pool
}

// NewPGDashboardRepository cria um PGDashboardRepository.
func NewPGDashboardRepository(pool *pgxpool.Pool) *PGDashboardRepository {
	return &PGDashboardRepository{pool: pool}
}

// GetConsolidated retorna métricas agregadas de todos os vendedores.
// Task 6.1.2 — query com bind params (CHK020).
func (r *PGDashboardRepository) GetConsolidated(ctx context.Context, filter DashboardFilter) (ConsolidatedDashboard, error) {
	// ── 1. Totais de vendas (pedidos pagos) ──────────────────────────────────
	salesWhere, salesArgs := buildOrderPeriodWhere(filter, "o", 1)
	salesQuery := fmt.Sprintf(`
		SELECT
			COALESCE(SUM(o.total_cents), 0) AS total_sales_cents,
			COUNT(o.id)                     AS order_count,
			COUNT(DISTINCT o.vendor_id)     AS vendor_count
		FROM orders o
		WHERE o.status = 'pago' %s`, salesWhere)

	var dash ConsolidatedDashboard
	err := r.pool.QueryRow(ctx, salesQuery, salesArgs...).Scan(
		&dash.TotalSalesCents,
		&dash.OrderCount,
		&dash.VendorCount,
	)
	if err != nil {
		return ConsolidatedDashboard{}, fmt.Errorf("dashboard consolidated sales: %w", err)
	}

	// ── 2. Totais de comissões por status (usando view commission_net_balance) ─
	commWhere, commArgs := buildCommissionPeriodWhere(filter, "nb", 1)
	commQuery := fmt.Sprintf(`
		SELECT
			COALESCE(SUM(CASE WHEN nb.status = 'pendente'  THEN nb.net_cents ELSE 0 END), 0) AS pending,
			COALESCE(SUM(CASE WHEN nb.status = 'aprovado'  THEN nb.net_cents ELSE 0 END), 0) AS approved,
			COALESCE(SUM(CASE WHEN nb.status = 'pago'      THEN nb.net_cents ELSE 0 END), 0) AS paid,
			COALESCE(SUM(nb.net_cents), 0)                                                    AS total
		FROM commission_net_balance nb
		WHERE 1=1 %s`, commWhere)

	err = r.pool.QueryRow(ctx, commQuery, commArgs...).Scan(
		&dash.PendingCommCents,
		&dash.ApprovedCommCents,
		&dash.PaidCommCents,
		&dash.TotalCommCents,
	)
	if err != nil {
		return ConsolidatedDashboard{}, fmt.Errorf("dashboard consolidated commissions: %w", err)
	}

	// ── 3. Ranking top 10 vendedores por volume de vendas ────────────────────
	rankWhere, rankArgs := buildOrderPeriodWhere(filter, "o", 1)
	rankQuery := fmt.Sprintf(`
		SELECT
			o.vendor_id,
			v.name                          AS vendor_name,
			COALESCE(SUM(o.total_cents), 0) AS total_cents,
			COUNT(o.id)                     AS order_count,
			COALESCE(SUM(nb.net_cents), 0)  AS comm_cents
		FROM orders o
		JOIN vendors v ON v.id = o.vendor_id
		LEFT JOIN commission_net_balance nb ON nb.order_id = o.id
		WHERE o.status = 'pago' %s
		GROUP BY o.vendor_id, v.name
		ORDER BY total_cents DESC
		LIMIT 10`, rankWhere)

	rows, err := r.pool.Query(ctx, rankQuery, rankArgs...)
	if err != nil {
		return ConsolidatedDashboard{}, fmt.Errorf("dashboard ranking: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var vr VendorRanking
		if err := rows.Scan(&vr.VendorID, &vr.VendorName, &vr.TotalCents, &vr.OrderCount, &vr.CommCents); err != nil {
			return ConsolidatedDashboard{}, fmt.Errorf("dashboard ranking scan: %w", err)
		}
		dash.TopVendors = append(dash.TopVendors, vr)
	}
	if err := rows.Err(); err != nil {
		return ConsolidatedDashboard{}, fmt.Errorf("dashboard ranking rows: %w", err)
	}

	return dash, nil
}

// GetVendorDashboard retorna métricas de um único vendedor.
// Task 6.1.3 — escopo de vendedor garantido no service (P-IV).
func (r *PGDashboardRepository) GetVendorDashboard(ctx context.Context, vendorID string, filter DashboardFilter) (VendorDashboard, error) {
	// Sobrescreve o filter.VendorID para garantir o escopo (P-IV defense-in-depth).
	filter.VendorID = vendorID

	salesWhere, salesArgs := buildOrderPeriodWhere(filter, "o", 2)
	salesQuery := fmt.Sprintf(`
		SELECT
			COALESCE(SUM(o.total_cents), 0) AS total_sales_cents,
			COUNT(o.id)                     AS order_count
		FROM orders o
		WHERE o.vendor_id = $1 AND o.status = 'pago' %s`, salesWhere)

	args := append([]interface{}{vendorID}, salesArgs...)

	var vd VendorDashboard
	vd.VendorID = vendorID

	err := r.pool.QueryRow(ctx, salesQuery, args...).Scan(
		&vd.TotalSalesCents,
		&vd.OrderCount,
	)
	if err != nil {
		return VendorDashboard{}, fmt.Errorf("vendor dashboard sales: %w", err)
	}

	// Comissões por status
	commWhere, commArgs := buildCommissionPeriodWhere(filter, "nb", 2)
	commQuery := fmt.Sprintf(`
		SELECT
			COALESCE(SUM(CASE WHEN nb.status = 'pendente'  THEN nb.net_cents ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN nb.status = 'aprovado'  THEN nb.net_cents ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN nb.status = 'pago'      THEN nb.net_cents ELSE 0 END), 0),
			COALESCE(SUM(nb.net_cents), 0)
		FROM commission_net_balance nb
		WHERE nb.vendor_id = $1 %s`, commWhere)

	commArgs = append([]interface{}{vendorID}, commArgs...)
	err = r.pool.QueryRow(ctx, commQuery, commArgs...).Scan(
		&vd.PendingCommCents,
		&vd.ApprovedCommCents,
		&vd.PaidCommCents,
		&vd.TotalCommCents,
	)
	if err != nil {
		return VendorDashboard{}, fmt.Errorf("vendor dashboard commissions: %w", err)
	}

	return vd, nil
}

// GetDrillDown retorna rastreabilidade completa: pedido → comissões → estornos.
// Task 6.1.4 — SC-004.
func (r *PGDashboardRepository) GetDrillDown(ctx context.Context, orderID string) (DrillDown, error) {
	// Pedido
	orderRow := r.pool.QueryRow(ctx, `
		SELECT id, vendor_id, total_cents, order_date, status, created_at
		FROM orders
		WHERE id = $1`, orderID)

	var dd DrillDown
	if err := orderRow.Scan(&dd.OrderID, &dd.VendorID, &dd.TotalCents, &dd.OrderDate, &dd.Status, &dd.CreatedAt); err != nil {
		return DrillDown{}, fmt.Errorf("drilldown order: %w", err)
	}

	// Comissões do pedido com saldo líquido
	commRows, err := r.pool.Query(ctx, `
		SELECT
			c.id,
			c.value_cents,
			COALESCE(nb.net_cents, c.value_cents) AS net_cents,
			c.status,
			c.period_year,
			c.period_month
		FROM commissions c
		LEFT JOIN commission_net_balance nb ON nb.commission_id = c.id
		WHERE c.order_id = $1
		ORDER BY c.calculated_at`, orderID)
	if err != nil {
		return DrillDown{}, fmt.Errorf("drilldown commissions: %w", err)
	}
	defer commRows.Close()

	for commRows.Next() {
		var dc DrillDownCommission
		if err := commRows.Scan(&dc.CommissionID, &dc.ValueCents, &dc.NetCents, &dc.Status, &dc.PeriodYear, &dc.PeriodMonth); err != nil {
			return DrillDown{}, fmt.Errorf("drilldown commission scan: %w", err)
		}

		// Estornos desta comissão
		revRows, err := r.pool.Query(ctx, `
			SELECT id, value_cents, status, created_at, actor_user_id
			FROM commission_reversals
			WHERE commission_id = $1
			ORDER BY created_at`, dc.CommissionID)
		if err != nil {
			return DrillDown{}, fmt.Errorf("drilldown reversals: %w", err)
		}

		for revRows.Next() {
			var dr DrillDownReversal
			if err := revRows.Scan(&dr.ReversalID, &dr.ValueCents, &dr.Status, &dr.CreatedAt, &dr.ActorUserID); err != nil {
				revRows.Close()
				return DrillDown{}, fmt.Errorf("drilldown reversal scan: %w", err)
			}
			dc.Reversals = append(dc.Reversals, dr)
		}
		revRows.Close()
		if err := revRows.Err(); err != nil {
			return DrillDown{}, fmt.Errorf("drilldown reversals rows: %w", err)
		}

		dd.Commissions = append(dd.Commissions, dc)
	}
	if err := commRows.Err(); err != nil {
		return DrillDown{}, fmt.Errorf("drilldown commissions rows: %w", err)
	}

	return dd, nil
}

// GetPendingCommissionsSummary retorna indicadores de comissões pendentes (FR-019).
// Task 6.2.4
func (r *PGDashboardRepository) GetPendingCommissionsSummary(ctx context.Context) (PendingCommissionsSummary, error) {
	var s PendingCommissionsSummary
	err := r.pool.QueryRow(ctx, `
		SELECT
			COUNT(CASE WHEN nb.status = 'pendente' THEN 1 END),
			COALESCE(SUM(CASE WHEN nb.status = 'pendente' THEN nb.net_cents END), 0),
			COUNT(CASE WHEN nb.status = 'aprovado' THEN 1 END),
			COALESCE(SUM(CASE WHEN nb.status = 'aprovado' THEN nb.net_cents END), 0)
		FROM commission_net_balance nb`).Scan(
		&s.PendingApprovalCount,
		&s.PendingApprovalCents,
		&s.ApprovedUnpaidCount,
		&s.ApprovedUnpaidCents,
	)
	if err != nil {
		return PendingCommissionsSummary{}, fmt.Errorf("pending commissions summary: %w", err)
	}
	return s, nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// buildOrderPeriodWhere constrói o trecho WHERE de filtro de período para a tabela orders.
// Orders usa order_date (DATE) para período — não tem period_year/period_month.
// CHK020: zero interpolação de strings com valores do usuário.
func buildOrderPeriodWhere(filter DashboardFilter, tableAlias string, nextArg int) (string, []interface{}) {
	var where string
	var args []interface{}
	n := nextArg

	if filter.Year > 0 {
		// Filtra por EXTRACT para year+month em order_date (DATE column)
		where += fmt.Sprintf(" AND EXTRACT(YEAR FROM %s.order_date) = $%d", tableAlias, n)
		args = append(args, filter.Year)
		n++
		if filter.Month > 0 {
			where += fmt.Sprintf(" AND EXTRACT(MONTH FROM %s.order_date) = $%d", tableAlias, n)
			args = append(args, filter.Month)
			n++
		}
	} else if filter.StartDate != nil {
		where += fmt.Sprintf(" AND %s.order_date >= $%d", tableAlias, n)
		args = append(args, filter.StartDate)
		n++
		if filter.EndDate != nil {
			where += fmt.Sprintf(" AND %s.order_date <= $%d", tableAlias, n)
			args = append(args, filter.EndDate)
			n++
		}
	}
	_ = n
	return where, args
}

// buildCommissionPeriodWhere constrói WHERE de filtro para comissões/view.
// Usa period_year/period_month (colunas nativas das commissions).
// CHK020: zero interpolação de strings com valores do usuário.
func buildCommissionPeriodWhere(filter DashboardFilter, tableAlias string, nextArg int) (string, []interface{}) {
	var where string
	var args []interface{}
	n := nextArg

	if filter.Year > 0 {
		where += fmt.Sprintf(" AND %s.period_year = $%d", tableAlias, n)
		args = append(args, filter.Year)
		n++
		if filter.Month > 0 {
			where += fmt.Sprintf(" AND %s.period_month = $%d", tableAlias, n)
			args = append(args, filter.Month)
			n++
		}
	}
	_ = n
	return where, args
}
