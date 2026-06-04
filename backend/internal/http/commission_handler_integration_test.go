//go:build integration

// Package http_test — testes de integração HTTP para CommissionHandler.
// Task 5.1.5 + 5.3.5: apuração, idempotência SC-003, RBAC P-IV, escopo do vendedor.
// Requer PostgreSQL em localhost:5433.
//
// Executar com:
//
//	go test -tags=integration ./internal/http/... -run TestCommInteg -v
package http_test

import (
	"bytes"
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

// IDs fixos para testes de commission — distintos dos outros testes.
const (
	commIntegVendorID  = "33333333-3333-3333-3333-000000000001"
	commIntegVendor2ID = "33333333-3333-3333-3333-000000000002"
	commIntegActorID   = "44444444-4444-4444-4444-000000000001"
	commIntegUserID    = "55555555-5555-5555-5555-000000000001"
)

// setupCommIntegHTTP cria router com CommissionHandler conectado ao banco real.
func setupCommIntegHTTP(t *testing.T) (*chi.Mux, *pgxpool.Pool, func(role, vendorID string) string) {
	t.Helper()
	t.Setenv("JWT_SECRET", testSecret)

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDBURL)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	// Cleanup antes e depois dos testes
	cleanupCommInteg(pool)
	t.Cleanup(func() { cleanupCommInteg(pool) })

	// Criar usuário gestor/financeiro para audit_trail FK
	pool.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role)
		VALUES ($1, 'Test User Comm', 'test-comm-user@test.com', 'x', 'financeiro')
		ON CONFLICT (id) DO NOTHING
	`, commIntegUserID)

	// Instanciar repositórios e serviços
	orderRepo := repository.NewPGOrderRepository(pool)
	commRepo := repository.NewPGCommissionRepository(pool)
	ruleRepo := repository.NewPGCommissionRuleRepository(pool)
	apurationSvc := service.NewApurationService(orderRepo, ruleRepo, commRepo)
	handler := apphttp.NewCommissionHandler(apurationSvc)

	blocklist := appauth.NewDBBlocklist(pool)
	authVerifier := func(r *http.Request, tokenString string) (*appauth.Claims, error) {
		return appauth.VerifyTokenWithBlocklist(r.Context(), tokenString, blocklist)
	}

	// Emitir token para o teste
	issueToken := func(role, vendorID string) string {
		tok, err := appauth.IssueAccessToken(commIntegUserID, role, vendorID)
		require.NoError(t, err)
		return tok
	}

	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.RequireAuth(authVerifier))
		r.With(middleware.RequireRole("gestor")).Post("/commissions/apurate", handler.Apurate)
		r.With(middleware.RequireRole("gestor", "financeiro", "vendedor")).Get("/commissions", handler.List)
		r.With(middleware.RequireRole("gestor", "financeiro", "vendedor")).Get("/commissions/{id}", handler.Get)
		r.With(middleware.RequireRole("financeiro")).Patch("/commissions/{id}/status", handler.Transition)
	})

	return r, pool, issueToken
}

func cleanupCommInteg(pool *pgxpool.Pool) {
	ctx := context.Background()
	pool.Exec(ctx, `SET session_replication_role = 'replica'`)
	for _, vid := range []string{commIntegVendorID, commIntegVendor2ID} {
		pool.Exec(ctx, `DELETE FROM commission_reversals WHERE commission_id IN (SELECT id FROM commissions WHERE vendor_id = $1)`, vid)
		pool.Exec(ctx, `DELETE FROM audit_trail WHERE entity_id IN (SELECT id FROM commissions WHERE vendor_id = $1)`, vid)
		pool.Exec(ctx, `DELETE FROM commissions WHERE vendor_id = $1`, vid)
		pool.Exec(ctx, `DELETE FROM audit_trail WHERE entity_id IN (SELECT id FROM orders WHERE vendor_id = $1)`, vid)
		pool.Exec(ctx, `DELETE FROM orders WHERE vendor_id = $1`, vid)
		pool.Exec(ctx, `DELETE FROM commission_rules WHERE vendor_id = $1`, vid)
		pool.Exec(ctx, `DELETE FROM users WHERE vendor_id = $1`, vid)
		pool.Exec(ctx, `DELETE FROM vendors WHERE id = $1`, vid)
	}
	pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, commIntegUserID)
	pool.Exec(ctx, `SET session_replication_role = 'origin'`)
}

// seedCommIntegData cria vendedor + regra + pedido pago para os testes.
func seedCommIntegData(t *testing.T, pool *pgxpool.Pool, vendorID string, percentage decimal.Decimal, orderTotalCents int64, orderDate time.Time) string {
	t.Helper()
	ctx := context.Background()

	// Criar vendedor — email derivado com string formatting (sem concatenação SQL)
	email := vendorID + "@commtest.com"
	_, err := pool.Exec(ctx, `
		INSERT INTO vendors (id, name, email, status)
		VALUES ($1, 'Vendedor Teste Comm', $2, 'ativo')
		ON CONFLICT (id) DO NOTHING
	`, vendorID, email)
	require.NoError(t, err)

	// Criar regra de comissão
	validFrom := orderDate.AddDate(0, -1, 0).Format("2006-01-02")
	_, err = pool.Exec(ctx, `
		INSERT INTO commission_rules (vendor_id, percentage, valid_from, version)
		VALUES ($1, $2, $3, 1)
		ON CONFLICT DO NOTHING
	`, vendorID, percentage.String(), validFrom)
	require.NoError(t, err)

	// Criar pedido pago
	var orderID string
	err = pool.QueryRow(ctx, `
		INSERT INTO orders (vendor_id, total_cents, order_date, status, paid_at)
		VALUES ($1, $2, $3, 'pago', NOW())
		RETURNING id
	`, vendorID, orderTotalCents, orderDate.Format("2006-01-02")).Scan(&orderID)
	require.NoError(t, err)

	return orderID
}

// ─── 5.3.5 — Testes de integração HTTP ────────────────────────────────────────

// TestCommInteg_ApurateOK — apurar período → 200 com resultado correto.
func TestCommInteg_ApurateOK(t *testing.T) {
	r, pool, issueToken := setupCommIntegHTTP(t)

	// Seed: vendedor 8% + pedido R$5000 em junho/2026
	orderDate := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	seedCommIntegData(t, pool, commIntegVendorID, decimal.NewFromFloat(8), 500000, orderDate)

	body := `{"year":2026,"month":6}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/commissions/apurate",
		bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+issueToken("gestor", ""))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "apuração deve retornar 200")

	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, float64(2026), resp["periodYear"])
	assert.Equal(t, float64(6), resp["periodMonth"])
	assert.Equal(t, float64(1), resp["calculated"])
	assert.Equal(t, float64(0), resp["skipped"])
	// 8% de R$5000 = R$400 = 40000 centavos
	assert.Equal(t, float64(40000), resp["totalCents"], "8%% de R$5000 deve ser R$400 (40000 centavos)")
}

// TestCommInteg_ApurateForbiddenForVendedor — Vendedor tentando apurar → 403.
func TestCommInteg_ApurateForbiddenForVendedor(t *testing.T) {
	r, _, issueToken := setupCommIntegHTTP(t)

	body := `{"year":2026,"month":6}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/commissions/apurate",
		bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+issueToken("vendedor", commIntegVendorID))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code, "vendedor não pode apurar")
}

// TestCommInteg_ApurateIdempotent_SC003 — reapurar o mesmo período → skipped=N, calculated=0.
func TestCommInteg_ApurateIdempotent_SC003(t *testing.T) {
	r, pool, issueToken := setupCommIntegHTTP(t)

	// Use month 7 to isolate from TestCommInteg_ApurateOK (which uses month 6)
	orderDate := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	seedCommIntegData(t, pool, commIntegVendorID, decimal.NewFromFloat(10), 100000, orderDate)

	body := `{"year":2026,"month":7}`
	headers := func() *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/commissions/apurate",
			bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+issueToken("gestor", ""))
		return req
	}

	// Primeira apuração
	rr1 := httptest.NewRecorder()
	r.ServeHTTP(rr1, headers())
	assert.Equal(t, http.StatusOK, rr1.Code)

	var resp1 map[string]interface{}
	require.NoError(t, json.NewDecoder(rr1.Body).Decode(&resp1))
	assert.Equal(t, float64(1), resp1["calculated"])

	// Segunda apuração — deve ser idempotente
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, headers())
	assert.Equal(t, http.StatusOK, rr2.Code)

	var resp2 map[string]interface{}
	require.NoError(t, json.NewDecoder(rr2.Body).Decode(&resp2))
	assert.Equal(t, float64(0), resp2["calculated"], "re-apuração não deve inserir duplicatas (SC-003)")
	assert.Equal(t, float64(1), resp2["skipped"], "pedido já existente deve ser skipped")
}

// TestCommInteg_ListCommissions_VendorScope — Vendedor lista → apenas suas comissões.
func TestCommInteg_ListCommissions_VendorScope(t *testing.T) {
	r, pool, issueToken := setupCommIntegHTTP(t)

	// Seed para dois vendedores
	orderDate := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	seedCommIntegData(t, pool, commIntegVendorID, decimal.NewFromFloat(8), 200000, orderDate)
	seedCommIntegData(t, pool, commIntegVendor2ID, decimal.NewFromFloat(10), 100000, orderDate)

	// Apurar para ambos
	body := `{"year":2026,"month":6}`
	apReq := httptest.NewRequest(http.MethodPost, "/api/v1/commissions/apurate",
		bytes.NewBufferString(body))
	apReq.Header.Set("Content-Type", "application/json")
	apReq.Header.Set("Authorization", "Bearer "+issueToken("gestor", ""))
	apRR := httptest.NewRecorder()
	r.ServeHTTP(apRR, apReq)
	require.Equal(t, http.StatusOK, apRR.Code)

	// Vendedor 1 lista → deve ver apenas suas comissões
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/commissions?year=2026&month=6", nil)
	listReq.Header.Set("Authorization", "Bearer "+issueToken("vendedor", commIntegVendorID))
	listRR := httptest.NewRecorder()
	r.ServeHTTP(listRR, listReq)
	assert.Equal(t, http.StatusOK, listRR.Code)

	var commissions []map[string]interface{}
	require.NoError(t, json.NewDecoder(listRR.Body).Decode(&commissions))

	for _, c := range commissions {
		assert.Equal(t, commIntegVendorID, c["vendorId"],
			"vendedor não deve ver comissões de outro vendedor (SC-005/P-IV)")
	}
}

// TestCommInteg_TransitionCommission_Financeiro — Financeiro aprova comissão → 200.
func TestCommInteg_TransitionCommission_Financeiro(t *testing.T) {
	r, pool, issueToken := setupCommIntegHTTP(t)

	// Seed + apurar
	orderDate := time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC)
	seedCommIntegData(t, pool, commIntegVendorID, decimal.NewFromFloat(8), 300000, orderDate)

	body := `{"year":2026,"month":6}`
	apReq := httptest.NewRequest(http.MethodPost, "/api/v1/commissions/apurate",
		bytes.NewBufferString(body))
	apReq.Header.Set("Content-Type", "application/json")
	apReq.Header.Set("Authorization", "Bearer "+issueToken("gestor", ""))
	apRR := httptest.NewRecorder()
	r.ServeHTTP(apRR, apReq)
	require.Equal(t, http.StatusOK, apRR.Code)

	// Buscar a comissão criada
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/commissions?year=2026&month=6", nil)
	listReq.Header.Set("Authorization", "Bearer "+issueToken("financeiro", ""))
	listRR := httptest.NewRecorder()
	r.ServeHTTP(listRR, listReq)
	require.Equal(t, http.StatusOK, listRR.Code)

	var commissions []map[string]interface{}
	require.NoError(t, json.NewDecoder(listRR.Body).Decode(&commissions))
	require.NotEmpty(t, commissions, "deve ter pelo menos uma comissão")

	commID := commissions[0]["id"].(string)
	assert.Equal(t, "pendente", commissions[0]["status"])

	// Financeiro aprova
	transBody := `{"status":"aprovado"}`
	transReq := httptest.NewRequest(http.MethodPatch, "/api/v1/commissions/"+commID+"/status",
		bytes.NewBufferString(transBody))
	transReq.Header.Set("Content-Type", "application/json")
	transReq.Header.Set("Authorization", "Bearer "+issueToken("financeiro", ""))
	transRR := httptest.NewRecorder()
	r.ServeHTTP(transRR, transReq)
	assert.Equal(t, http.StatusOK, transRR.Code, "financeiro deve poder aprovar comissão")

	var updated map[string]interface{}
	require.NoError(t, json.NewDecoder(transRR.Body).Decode(&updated))
	assert.Equal(t, "aprovado", updated["status"])
}

// TestCommInteg_TransitionCommission_ForbiddenForGestor — Gestor tentando transicionar → 403.
func TestCommInteg_TransitionCommission_ForbiddenForGestor(t *testing.T) {
	r, pool, issueToken := setupCommIntegHTTP(t)

	// Seed + apurar
	orderDate := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)
	seedCommIntegData(t, pool, commIntegVendorID, decimal.NewFromFloat(8), 100000, orderDate)

	body := `{"year":2026,"month":6}`
	apReq := httptest.NewRequest(http.MethodPost, "/api/v1/commissions/apurate",
		bytes.NewBufferString(body))
	apReq.Header.Set("Content-Type", "application/json")
	apReq.Header.Set("Authorization", "Bearer "+issueToken("gestor", ""))
	apRR := httptest.NewRecorder()
	r.ServeHTTP(apRR, apReq)
	require.Equal(t, http.StatusOK, apRR.Code)

	// Buscar a comissão
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/commissions", nil)
	listReq.Header.Set("Authorization", "Bearer "+issueToken("gestor", ""))
	listRR := httptest.NewRecorder()
	r.ServeHTTP(listRR, listReq)
	var commissions []map[string]interface{}
	require.NoError(t, json.NewDecoder(listRR.Body).Decode(&commissions))
	require.NotEmpty(t, commissions)
	commID := commissions[0]["id"].(string)

	// Gestor tenta transicionar → 403 (RBAC no nível de middleware)
	transReq := httptest.NewRequest(http.MethodPatch, "/api/v1/commissions/"+commID+"/status",
		bytes.NewBufferString(`{"status":"aprovado"}`))
	transReq.Header.Set("Content-Type", "application/json")
	transReq.Header.Set("Authorization", "Bearer "+issueToken("gestor", ""))
	transRR := httptest.NewRecorder()
	r.ServeHTTP(transRR, transReq)
	assert.Equal(t, http.StatusForbidden, transRR.Code, "gestor não pode transicionar comissão (P-IV)")
}

// TestCommInteg_CommissionNetBalance_WithReversal — commission_net_balance correto com estorno.
// 5.1.5: verificar que a view retorna net_cents correto após estorno.
func TestCommInteg_CommissionNetBalance_WithReversal(t *testing.T) {
	_, pool, _ := setupCommIntegHTTP(t)
	ctx := context.Background()

	// Seed manual: criar comissão e estorno diretamente
	orderDate := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)
	orderID := seedCommIntegData(t, pool, commIntegVendorID, decimal.NewFromFloat(10), 100000, orderDate)

	// Inserir comissão manualmente (bypassing service para controle preciso)
	var ruleID string
	err := pool.QueryRow(ctx, `
		SELECT id FROM commission_rules WHERE vendor_id = $1 LIMIT 1
	`, commIntegVendorID).Scan(&ruleID)
	require.NoError(t, err)

	var commID string
	err = pool.QueryRow(ctx, `
		INSERT INTO commissions (order_id, vendor_id, value_cents, applied_percentage, rule_id, period_year, period_month)
		VALUES ($1, $2, 10000, '10.0000', $3, 2026, 6)
		RETURNING id
	`, orderID, commIntegVendorID, ruleID).Scan(&commID)
	require.NoError(t, err)

	// Inserir estorno (value_cents negativo) — usando session_replication_role para bypass do trigger
	_, err = pool.Exec(ctx, `SET session_replication_role = 'replica'`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO commission_reversals (commission_id, order_id, value_cents, status, actor_user_id)
		VALUES ($1, $2, -3000, 'aplicado', $3)
	`, commID, orderID, commIntegActorID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `SET session_replication_role = 'origin'`)
	require.NoError(t, err)

	// Verificar net_cents via view (coluna chave é commission_id, não id — ver migration 009)
	var netCents int64
	err = pool.QueryRow(ctx, `
		SELECT net_cents FROM commission_net_balance WHERE commission_id = $1
	`, commID).Scan(&netCents)
	require.NoError(t, err)
	assert.Equal(t, int64(7000), netCents, "net_cents deve ser value_cents + reversals (10000 - 3000 = 7000)")
}
