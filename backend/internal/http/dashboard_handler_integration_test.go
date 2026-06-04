//go:build integration

// Package http_test — testes de integração HTTP para DashboardHandler.
// Task 6.2.6: dashboard consolidado carrega em < 3s; Vendedor acessando /consolidated → 403;
//             Vendedor vê apenas os próprios dados em /dashboard/vendor;
//             /drilldown retorna rastreabilidade (SC-004).
//
// Executar com:
//
//	go test -tags=integration ./internal/http/... -run TestDashInteg -v
package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appauth "financial-dashboard/backend/internal/auth"
	apphttp "financial-dashboard/backend/internal/http"
	"financial-dashboard/backend/internal/http/middleware"
	"financial-dashboard/backend/internal/repository"
	"financial-dashboard/backend/internal/service"
)

// IDs fixos para testes de dashboard HTTP — distintos dos outros suites.
const (
	dashHTTPVendorID  = "77777777-7777-7777-7777-000000000001"
	dashHTTPVendor2ID = "77777777-7777-7777-7777-000000000002"
	dashHTTPUserID    = "77777777-7777-7777-7777-000000000099"
)

// setupDashIntegHTTP cria router com DashboardHandler conectado ao banco real.
func setupDashIntegHTTP(t *testing.T) (*chi.Mux, *pgxpool.Pool, func(role, vendorID string) string) {
	t.Helper()
	t.Setenv("JWT_SECRET", testSecret)

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDBURL)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	cleanupDashInteg(pool)
	t.Cleanup(func() { cleanupDashInteg(pool) })

	// Usuário ator
	_, _ = pool.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role)
		VALUES ($1, 'Dash HTTP User', 'dash-http@test.com', 'x', 'gestor')
		ON CONFLICT (id) DO NOTHING`, dashHTTPUserID)

	// Dois vendedores
	for _, v := range []struct{ id, name, email string }{
		{dashHTTPVendorID, "Dash HTTP Vendor 1", "dash-http-v1@test.com"},
		{dashHTTPVendor2ID, "Dash HTTP Vendor 2", "dash-http-v2@test.com"},
	} {
		_, _ = pool.Exec(ctx, `
			INSERT INTO vendors (id, name, email, status)
			VALUES ($1, $2, $3, 'ativo')
			ON CONFLICT (id) DO NOTHING`, v.id, v.name, v.email)
	}

	dashRepo := repository.NewPGDashboardRepository(pool)
	dashSvc := service.NewDashboardService(dashRepo)
	dashHandler := apphttp.NewDashboardHandler(dashSvc)

	blocklist := appauth.NewDBBlocklist(pool)
	authVerifier := func(r *http.Request, tokenString string) (*appauth.Claims, error) {
		return appauth.VerifyTokenWithBlocklist(r.Context(), tokenString, blocklist)
	}

	issueToken := func(role, vendorID string) string {
		tok, err := appauth.IssueAccessToken(dashHTTPUserID, role, vendorID)
		require.NoError(t, err)
		return tok
	}

	router := chi.NewRouter()
	router.Group(func(r chi.Router) {
		r.Use(middleware.RequireAuth(authVerifier))
		r.With(middleware.RequireRole("gestor", "financeiro")).Get("/api/v1/dashboard/consolidated", dashHandler.Consolidated)
		r.With(middleware.RequireRole("gestor", "financeiro", "vendedor")).Get("/api/v1/dashboard/vendor", dashHandler.VendorDashboard)
		r.With(middleware.RequireRole("gestor", "financeiro")).Get("/api/v1/dashboard/commissions/pending", dashHandler.PendingCommissions)
		r.Get("/api/v1/orders/{id}/drilldown", dashHandler.DrillDown)
	})

	return router, pool, issueToken
}

// cleanupDashInteg limpa dados de teste do dashboard HTTP.
func cleanupDashInteg(pool *pgxpool.Pool) {
	ctx := context.Background()
	_, _ = pool.Exec(ctx, `
		DO $$ BEGIN
			SET LOCAL session_replication_role = 'replica';
			DELETE FROM commission_reversals WHERE commission_id IN (
				SELECT id FROM commissions WHERE vendor_id IN (
					'77777777-7777-7777-7777-000000000001',
					'77777777-7777-7777-7777-000000000002'
				)
			);
			DELETE FROM commissions WHERE vendor_id IN (
				'77777777-7777-7777-7777-000000000001',
				'77777777-7777-7777-7777-000000000002'
			);
			DELETE FROM commission_rules WHERE vendor_id IN (
				'77777777-7777-7777-7777-000000000001',
				'77777777-7777-7777-7777-000000000002'
			);
			DELETE FROM order_items WHERE order_id IN (
				SELECT id FROM orders WHERE vendor_id IN (
					'77777777-7777-7777-7777-000000000001',
					'77777777-7777-7777-7777-000000000002'
				)
			);
			DELETE FROM orders WHERE vendor_id IN (
				'77777777-7777-7777-7777-000000000001',
				'77777777-7777-7777-7777-000000000002'
			);
			DELETE FROM vendors WHERE id IN (
				'77777777-7777-7777-7777-000000000001',
				'77777777-7777-7777-7777-000000000002'
			);
			DELETE FROM users WHERE id = '77777777-7777-7777-7777-000000000099';
		END $$
	`)
}

// insertDashHTTPOrder insere pedido 'pago' e retorna ID.
func insertDashHTTPOrder(t *testing.T, pool *pgxpool.Pool, vendorID string, totalCents int64) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(), `
		INSERT INTO orders (vendor_id, total_cents, order_date, status)
		VALUES ($1, $2, CURRENT_DATE, 'pago')
		RETURNING id`,
		vendorID, totalCents).Scan(&id)
	require.NoError(t, err)
	return id
}

// insertDashHTTPCommission insere comissão e retorna ID.
func insertDashHTTPCommission(t *testing.T, pool *pgxpool.Pool, vendorID, orderID string, valueCents int64, status string) string {
	t.Helper()
	ctx := context.Background()
	pct := decimal.NewFromFloat(5.0)
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
		VALUES ($1, $2, $3, $4, $5, 2026, 6, $6)
		RETURNING id`,
		vendorID, orderID, valueCents, pct, ruleID, status).Scan(&commID)
	require.NoError(t, err)
	return commID
}

// TestDashInteg_ConsolidatedVendedorForbidden garante que vendedor não pode acessar /consolidated.
// Task 6.2.6: Vendedor acessando dashboard consolidado → 403.
func TestDashInteg_ConsolidatedVendedorForbidden(t *testing.T) {
	router, _, issueToken := setupDashIntegHTTP(t)

	tok := issueToken("vendedor", dashHTTPVendorID)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/consolidated", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code, "vendedor deve receber 403 em /consolidated")
}

// TestDashInteg_ConsolidadoGestor verifica que gestor acessa /consolidated com sucesso.
func TestDashInteg_ConsolidadoGestor(t *testing.T) {
	router, pool, issueToken := setupDashIntegHTTP(t)

	// Inserir dados básicos
	o1 := insertDashHTTPOrder(t, pool, dashHTTPVendorID, 50000)
	insertDashHTTPCommission(t, pool, dashHTTPVendorID, o1, 2500, "pendente")

	tok := issueToken("gestor", "")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/consolidated?year=2026&month=6", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	// totalSalesCents deve incluir nosso pedido de 50000
	assert.GreaterOrEqual(t, body["totalSalesCents"], float64(50000))
}

// TestDashInteg_ConsolidadoPerformance verifica que /consolidated carrega em < 3s (SC-009).
// Task 6.2.6: dashboard com N pedidos carrega em < 3s.
func TestDashInteg_ConsolidadoPerformance(t *testing.T) {
	router, _, issueToken := setupDashIntegHTTP(t)

	tok := issueToken("gestor", "")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/consolidated", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()

	start := time.Now()
	router.ServeHTTP(rec, req)
	elapsed := time.Since(start)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Less(t, elapsed, 3*time.Second, "dashboard consolidado deve responder em < 3s (SC-009)")
}

// TestDashInteg_VendorDashboardEscopo verifica que vendedor só vê os próprios dados.
// P-IV: Vendedor vê apenas as próprias métricas.
func TestDashInteg_VendorDashboardEscopo(t *testing.T) {
	router, pool, issueToken := setupDashIntegHTTP(t)

	// Pedido e comissão do vendor1
	o1 := insertDashHTTPOrder(t, pool, dashHTTPVendorID, 10000)
	insertDashHTTPCommission(t, pool, dashHTTPVendorID, o1, 500, "pendente")

	// Pedido e comissão do vendor2
	o2 := insertDashHTTPOrder(t, pool, dashHTTPVendor2ID, 20000)
	insertDashHTTPCommission(t, pool, dashHTTPVendor2ID, o2, 1000, "aprovado")

	// Vendedor 1 acessa /dashboard/vendor — deve ver só seus dados
	tok := issueToken("vendedor", dashHTTPVendorID)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/vendor", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))

	// totalSalesCents só do vendor1 (10000) — não inclui vendor2 (20000)
	assert.Equal(t, float64(10000), body["totalSalesCents"], "vendedor só vê os próprios dados (P-IV)")
}

// TestDashInteg_DrilldownRastreabilidade verifica SC-004: pedido → comissão → estorno.
func TestDashInteg_DrilldownRastreabilidade(t *testing.T) {
	router, pool, issueToken := setupDashIntegHTTP(t)

	orderID := insertDashHTTPOrder(t, pool, dashHTTPVendorID, 10000)
	commID := insertDashHTTPCommission(t, pool, dashHTTPVendorID, orderID, 500, "pendente")

	// Inserir estorno (commission_reversals requer order_id e actor_user_id NOT NULL)
	_, err := pool.Exec(context.Background(), `
		INSERT INTO commission_reversals (commission_id, order_id, value_cents, status, actor_user_id)
		VALUES ($1, $2, -100, 'aplicado', $3)`,
		commID, orderID, dashHTTPUserID)
	require.NoError(t, err)

	tok := issueToken("gestor", "")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+orderID+"/drilldown", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))

	assert.Equal(t, orderID, body["orderId"])
	assert.Equal(t, float64(10000), body["totalCents"])

	commList, ok := body["commissions"].([]interface{})
	require.True(t, ok, "deve ter lista de comissões")
	require.Len(t, commList, 1)

	commObj := commList[0].(map[string]interface{})
	assert.Equal(t, float64(500), commObj["valueCents"])
	// net = 500 + (-100) = 400
	assert.Equal(t, float64(400), commObj["netCents"], "net_cents após estorno")

	revList, ok := commObj["reversals"].([]interface{})
	require.True(t, ok)
	require.Len(t, revList, 1)
	revObj := revList[0].(map[string]interface{})
	assert.Equal(t, float64(-100), revObj["valueCents"])
	assert.NotEmpty(t, revObj["actorUserId"], "actorUserId deve ser auditável (P-I)")
}
