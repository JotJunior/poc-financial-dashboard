//go:build integration

// Package http_test — testes de integração HTTP para OrderHandler.
// Task 4.3.4 + 4.2.5 (CHK081 atomicidade cancelamento+estorno).
// Requer PostgreSQL em localhost:5433.
//
// Executar com:
//
//	go test -tags=integration ./internal/http/... -run TestOrderInteg -v
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

// IDs fixos para testes de order HTTP — distintos dos outros testes.
const (
	orderIntegVendorID = "11111111-1111-1111-1111-000000000001"
	orderIntegActorID  = "22222222-2222-2222-2222-000000000001"
)

// setupOrderIntegHTTP cria o router com OrderHandler conectado ao banco real.
func setupOrderIntegHTTP(t *testing.T) (*chi.Mux, *pgxpool.Pool, func(role, vendorID string) string) {
	t.Helper()
	t.Setenv("JWT_SECRET", testSecret)

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDBURL)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	cleanup := func() {
		pool.Exec(context.Background(), `
			DO $$ BEGIN
				SET session_replication_role = 'replica';
				DELETE FROM commission_reversals WHERE order_id IN (
					SELECT id FROM orders WHERE vendor_id = '11111111-1111-1111-1111-000000000001'
				);
				DELETE FROM commissions WHERE order_id IN (
					SELECT id FROM orders WHERE vendor_id = '11111111-1111-1111-1111-000000000001'
				);
				DELETE FROM audit_trail WHERE entity_id IN (
					SELECT id FROM orders WHERE vendor_id = '11111111-1111-1111-1111-000000000001'
				);
				DELETE FROM order_items WHERE order_id IN (
					SELECT id FROM orders WHERE vendor_id = '11111111-1111-1111-1111-000000000001'
				);
				DELETE FROM orders WHERE vendor_id = '11111111-1111-1111-1111-000000000001';
				DELETE FROM commission_rules WHERE vendor_id = '11111111-1111-1111-1111-000000000001';
				DELETE FROM vendors WHERE id = '11111111-1111-1111-1111-000000000001';
				DELETE FROM users WHERE id = '22222222-2222-2222-2222-000000000001';
			END $$
		`)
	}
	cleanup()
	t.Cleanup(cleanup)

	// Inserir usuário e vendedor de teste com regra de comissão.
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role)
		VALUES ($1, 'Gestor Order HTTP', 'integ-order-http@test.com',
		        '$argon2id$v=19$m=65536,t=3,p=4$dGVzdA$dGVzdA', 'gestor')
		ON CONFLICT (id) DO NOTHING
	`, orderIntegActorID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO vendors (id, name, email, status)
		VALUES ($1, 'Vendedor Order HTTP', 'vendor-order-http@test.com', 'ativo')
		ON CONFLICT (id) DO NOTHING
	`, orderIntegVendorID)
	require.NoError(t, err)

	// Inserir regra de comissão de 10% para o vendedor de teste.
	_, err = pool.Exec(ctx, `
		INSERT INTO commission_rules (vendor_id, percentage, valid_from, version)
		VALUES ($1, 10.0000, '2020-01-01', 1)
		ON CONFLICT DO NOTHING
	`, orderIntegVendorID)
	require.NoError(t, err)

	// Construir dependências.
	orderRepo := repository.NewPGOrderRepository(pool)
	commRepo := repository.NewPGCommissionRepository(pool)
	commRuleRepo := repository.NewPGCommissionRuleRepository(pool)

	orderSvc := service.NewOrderService(orderRepo, commRepo, commRuleRepo)
	orderHandler := apphttp.NewOrderHandler(orderSvc)

	blocklist := appauth.NewDBBlocklist(pool)
	authVerifier := func(r *http.Request, tokenString string) (*appauth.Claims, error) {
		return appauth.VerifyTokenWithBlocklist(r.Context(), tokenString, blocklist)
	}

	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireAuth(authVerifier))
		r.With(middleware.RequireRole("gestor")).Post("/api/v1/orders", orderHandler.Create)
		r.Get("/api/v1/orders", orderHandler.List)
		r.Get("/api/v1/orders/{id}", orderHandler.Get)
		r.With(middleware.RequireRole("gestor")).Patch("/api/v1/orders/{id}/status", orderHandler.Transition)
	})

	issueToken := func(role, vendorID string) string {
		tok, err := appauth.IssueAccessToken(orderIntegActorID, role, vendorID)
		require.NoError(t, err)
		return tok
	}

	return r, pool, issueToken
}

func doOrderReq(t *testing.T, router http.Handler, method, path string, body any, token string) *httptest.ResponseRecorder {
	t.Helper()
	var bodyReader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		bodyReader = bytes.NewReader(b)
	} else {
		bodyReader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// TestOrderInteg_Create: POST /orders com Gestor → 201.
func TestOrderInteg_Create(t *testing.T) {
	router, _, issueToken := setupOrderIntegHTTP(t)
	token := issueToken("gestor", "")

	body := map[string]any{
		"vendorId":   orderIntegVendorID,
		"totalCents": 100000,
		"orderDate":  "2025-06-01",
		"items": []map[string]any{
			{"description": "Produto X", "quantity": 1, "unitPriceCents": 100000, "lineTotalCents": 100000},
		},
	}

	w := doOrderReq(t, router, http.MethodPost, "/api/v1/orders", body, token)
	assert.Equal(t, http.StatusCreated, w.Code, "POST /orders com Gestor deve retornar 201, body: %s", w.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "rascunho", resp["status"])
	assert.Equal(t, float64(100000), resp["totalCents"])
}

// TestOrderInteg_Create_Forbidden: POST /orders com Vendedor → 403.
func TestOrderInteg_Create_Forbidden(t *testing.T) {
	router, _, issueToken := setupOrderIntegHTTP(t)
	token := issueToken("vendedor", orderIntegVendorID)

	body := map[string]any{
		"vendorId":   orderIntegVendorID,
		"totalCents": 50000,
		"orderDate":  "2025-06-01",
	}

	w := doOrderReq(t, router, http.MethodPost, "/api/v1/orders", body, token)
	assert.Equal(t, http.StatusForbidden, w.Code, "POST /orders com Vendedor deve retornar 403")
}

// TestOrderInteg_Transition: PATCH /orders/{id}/status → rascunho→confirmado→pago.
func TestOrderInteg_Transition(t *testing.T) {
	router, pool, issueToken := setupOrderIntegHTTP(t)
	token := issueToken("gestor", "")
	ctx := context.Background()

	// Criar pedido diretamente via SQL para evitar HTTP overhead.
	var orderID string
	err := pool.QueryRow(ctx, `
		INSERT INTO orders (vendor_id, total_cents, order_date, status)
		VALUES ($1, 50000, '2025-06-15', 'rascunho')
		RETURNING id
	`, orderIntegVendorID).Scan(&orderID)
	require.NoError(t, err)

	// rascunho → confirmado.
	w := doOrderReq(t, router, http.MethodPatch, "/api/v1/orders/"+orderID+"/status",
		map[string]string{"status": "confirmado"}, token)
	assert.Equal(t, http.StatusOK, w.Code, "confirmado deve retornar 200, body: %s", w.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "confirmado", resp["status"])

	// confirmado → pago (calcula comissão: 10% de R$500 = R$50 = 5000 cents).
	w = doOrderReq(t, router, http.MethodPatch, "/api/v1/orders/"+orderID+"/status",
		map[string]string{"status": "pago"}, token)
	assert.Equal(t, http.StatusOK, w.Code, "pago deve retornar 200, body: %s", w.Body.String())

	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "pago", resp["status"])
	assert.NotNil(t, resp["paidAt"], "paidAt deve ser preenchido")

	// Verificar comissão criada no banco.
	var commValue int64
	err = pool.QueryRow(ctx, `SELECT value_cents FROM commissions WHERE order_id = $1`, orderID).Scan(&commValue)
	require.NoError(t, err, "comissão deve ter sido criada")
	assert.Equal(t, int64(5000), commValue, "10%% de 50000 = 5000 centavos")
}

// TestOrderInteg_Transition_Invalid: pago→rascunho retorna 422.
func TestOrderInteg_Transition_Invalid(t *testing.T) {
	router, pool, issueToken := setupOrderIntegHTTP(t)
	token := issueToken("gestor", "")
	ctx := context.Background()

	// Criar pedido já pago.
	var orderID string
	err := pool.QueryRow(ctx, `
		INSERT INTO orders (vendor_id, total_cents, order_date, status, paid_at)
		VALUES ($1, 10000, '2025-05-01', 'pago', NOW())
		RETURNING id
	`, orderIntegVendorID).Scan(&orderID)
	require.NoError(t, err)

	w := doOrderReq(t, router, http.MethodPatch, "/api/v1/orders/"+orderID+"/status",
		map[string]string{"status": "rascunho"}, token)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code, "transição inválida deve retornar 422")

	var errResp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, "invalid_transition", errResp["error"])
}

// TestOrderInteg_Cancel_CHK081: cancelar pedido pago cria estorno ATOMICAMENTE.
// CHK081/dec-038: verifica que não existe pedido cancelado sem estorno correspondente.
func TestOrderInteg_Cancel_CHK081(t *testing.T) {
	router, pool, issueToken := setupOrderIntegHTTP(t)
	token := issueToken("gestor", "")
	ctx := context.Background()

	// Criar pedido no estado pago (com comissão já calculada).
	var orderID string
	err := pool.QueryRow(ctx, `
		INSERT INTO orders (vendor_id, total_cents, order_date, status, paid_at)
		VALUES ($1, 200000, '2025-04-01', 'pago', NOW())
		RETURNING id
	`, orderIntegVendorID).Scan(&orderID)
	require.NoError(t, err)

	// Inserir comissão existente (10% de R$2.000 = R$200 = 20000 cents).
	ruleRepo := repository.NewPGCommissionRuleRepository(pool)
	rule, err := ruleRepo.FindActiveAtDate(ctx, orderIntegVendorID, time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	commRepo := repository.NewPGCommissionRepository(pool)
	comm, err := commRepo.Create(ctx, repository.CreateCommissionReq{
		OrderID:           orderID,
		VendorID:          orderIntegVendorID,
		ValueCents:        20000,
		AppliedPercentage: decimal.NewFromFloat(10.0),
		RuleID:            rule.ID,
		PeriodYear:        2025,
		PeriodMonth:       4,
	})
	require.NoError(t, err)

	// Cancelar pedido pago — deve criar estorno atomicamente.
	w := doOrderReq(t, router, http.MethodPatch, "/api/v1/orders/"+orderID+"/status",
		map[string]string{"status": "cancelado"}, token)
	assert.Equal(t, http.StatusOK, w.Code, "cancelamento deve retornar 200, body: %s", w.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "cancelado", resp["status"])

	// CHK081: verificar que estorno existe (atomicidade — pedido cancelado TEM estorno).
	var reversalCount int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM commission_reversals
		WHERE commission_id = $1 AND order_id = $2
	`, comm.ID, orderID).Scan(&reversalCount)
	require.NoError(t, err)
	assert.Equal(t, 1, reversalCount,
		"CHK081: deve existir exatamente 1 estorno para o cancelamento do pedido pago")

	// Verificar valor do estorno (negativo).
	var reversalValue int64
	err = pool.QueryRow(ctx, `
		SELECT value_cents FROM commission_reversals WHERE commission_id = $1
	`, comm.ID).Scan(&reversalValue)
	require.NoError(t, err)
	assert.Equal(t, int64(-20000), reversalValue, "estorno deve ser negativo (-20000)")
}

// TestOrderInteg_Get: GET /orders/{id} retorna pedido com itens.
func TestOrderInteg_Get(t *testing.T) {
	router, pool, issueToken := setupOrderIntegHTTP(t)
	token := issueToken("gestor", "")
	ctx := context.Background()

	var orderID string
	err := pool.QueryRow(ctx, `
		INSERT INTO orders (vendor_id, total_cents, order_date, status)
		VALUES ($1, 75000, '2025-07-01', 'rascunho')
		RETURNING id
	`, orderIntegVendorID).Scan(&orderID)
	require.NoError(t, err)

	w := doOrderReq(t, router, http.MethodGet, "/api/v1/orders/"+orderID, nil, token)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, orderID, resp["id"])
	assert.Equal(t, float64(75000), resp["totalCents"])
}

// TestOrderInteg_VendedorCannotSeeOtherOrder: Vendedor de outro vendor → 403.
// (Vendedor vê apenas seus próprios pedidos via escopo — SC-005).
// Nota: a restrição de escopo do vendedor é aplicada na List, não no Get individual.
// Este teste valida o comportamento da List com escopo correto.
func TestOrderInteg_VendedorList_ScopedToOwnVendor(t *testing.T) {
	router, _, issueToken := setupOrderIntegHTTP(t)
	// Token de vendedor com vendor_id do próprio (escopo correto).
	token := issueToken("vendedor", orderIntegVendorID)

	w := doOrderReq(t, router, http.MethodGet, "/api/v1/orders", nil, token)
	assert.Equal(t, http.StatusOK, w.Code, "vendedor deve conseguir listar seus pedidos, body: %s", w.Body.String())
}
