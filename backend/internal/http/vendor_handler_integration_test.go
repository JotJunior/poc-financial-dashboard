//go:build integration

// Testes de integração para task 3.4.5 — requer PostgreSQL em localhost:5433.
// Executar com: go test -tags=integration ./internal/http/... -run TestVendorInteg
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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"financial-dashboard/backend/internal/auth"
	apphttp "financial-dashboard/backend/internal/http"
	"financial-dashboard/backend/internal/http/middleware"
	"financial-dashboard/backend/internal/repository"
	"financial-dashboard/backend/internal/service"
)

// setupVendorInteg cria o router com VendorHandler conectado ao banco real.
func setupVendorInteg(t *testing.T) (*chi.Mux, *pgxpool.Pool, func(role, vendorID string) string) {
	t.Helper()
	t.Setenv("JWT_SECRET", testSecret)

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDBURL)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	// Limpar dados de testes anteriores (idempotência) e cleanup ao final.
	// Usa session_replication_role='replica' para contornar triggers de imutabilidade
	// (apenas em ambiente de teste — nunca em produção).
	cleanupVendors := func() {
		pool.Exec(context.Background(), `
			DO $$ BEGIN
				SET session_replication_role = 'replica';
				DELETE FROM audit_trail WHERE entity_type = 'vendor_anonymization'
					AND entity_id IN (SELECT id FROM vendors WHERE email LIKE '%integ-vendor%' OR email LIKE '%removed_%@anon.invalid' OR name LIKE 'REMOVED_%');
				DELETE FROM commission_rules WHERE vendor_id IN (
					SELECT id FROM vendors WHERE email LIKE '%integ-vendor%' OR email LIKE '%removed_%@anon.invalid' OR name LIKE 'REMOVED_%'
				);
				DELETE FROM vendors WHERE email LIKE '%integ-vendor%' OR email LIKE '%removed_%@anon.invalid' OR name LIKE 'REMOVED_%';
				SET session_replication_role = 'origin';
			END $$
		`)
	}
	cleanupVendors() // limpar antes (dados de runs anteriores)
	t.Cleanup(cleanupVendors)

	vendorRepo := repository.NewPGVendorRepository(pool)
	commissionRuleRepo := repository.NewPGCommissionRuleRepository(pool)
	vendorSvc := service.NewVendorService(vendorRepo, commissionRuleRepo, nil)
	vendorHandler := apphttp.NewVendorHandler(vendorSvc)

	blocklist := auth.NewDBBlocklist(pool)
	authVerifier := func(r *http.Request, tokenString string) (*auth.Claims, error) {
		return auth.VerifyTokenWithBlocklist(r.Context(), tokenString, blocklist)
	}

	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireAuth(authVerifier))
		r.With(middleware.RequireRole("gestor", "financeiro")).Get("/api/v1/vendors", vendorHandler.List)
		r.With(middleware.RequireRole("gestor")).Post("/api/v1/vendors", vendorHandler.Create)
		r.Get("/api/v1/vendors/{id}", vendorHandler.Get)
		r.With(middleware.RequireRole("gestor")).Patch("/api/v1/vendors/{id}", vendorHandler.Update)
		r.With(middleware.RequireRole("gestor")).Delete("/api/v1/vendors/{id}", vendorHandler.Delete)
	})

	// Inserir usuário ator (Gestor) para os testes que precisam do actor_user_id como UUID válido.
	actorID := "aaaaaaaa-aaaa-aaaa-aaaa-000000000001"
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role)
		VALUES ($1, 'Gestor Integração', 'integ-gestor-vendor@test.com', '$argon2id$v=19$m=65536,t=3,p=4$dGVzdA$dGVzdA', 'gestor')
		ON CONFLICT (email) DO NOTHING
	`, actorID)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM users WHERE email = 'integ-gestor-vendor@test.com'`)
	})

	// Helper: gerar token de acesso com papel e vendorID especificados.
	issueToken := func(role, vendorID string) string {
		tok, err := auth.IssueAccessToken(actorID, role, vendorID)
		require.NoError(t, err)
		return tok
	}

	return r, pool, issueToken
}

func doVendorReq(t *testing.T, router http.Handler, method, path string, body any, token string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
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

// TestVendorInteg_CreateGestor — POST /vendors com Gestor → 201.
func TestVendorInteg_CreateGestor(t *testing.T) {
	router, _, issueToken := setupVendorInteg(t)
	token := issueToken("gestor", "")

	w := doVendorReq(t, router, http.MethodPost, "/api/v1/vendors", map[string]string{
		"name":                 "Vendedor Teste Integ",
		"email":                "integ-vendor-create@test.com",
		"commissionPercentage": "8.5000",
	}, token)

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp["id"])
	assert.Equal(t, "Vendedor Teste Integ", resp["name"])
	assert.Equal(t, "ativo", resp["status"])
}

// TestVendorInteg_CreateVendedorForbidden — POST /vendors com Vendedor → 403.
func TestVendorInteg_CreateVendedorForbidden(t *testing.T) {
	router, _, issueToken := setupVendorInteg(t)
	token := issueToken("vendedor", "some-vendor-id")

	w := doVendorReq(t, router, http.MethodPost, "/api/v1/vendors", map[string]string{
		"name":                 "Nao Autorizado",
		"email":                "integ-vendor-forbidden@test.com",
		"commissionPercentage": "5.0000",
	}, token)

	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
}

// TestVendorInteg_GetGestor — GET /vendors/{id} com Gestor → 200.
func TestVendorInteg_GetGestor(t *testing.T) {
	router, pool, issueToken := setupVendorInteg(t)
	_ = pool

	// Criar um vendedor primeiro.
	gestorToken := issueToken("gestor", "")
	w := doVendorReq(t, router, http.MethodPost, "/api/v1/vendors", map[string]string{
		"name":                 "Vendedor Get Test",
		"email":                "integ-vendor-get@test.com",
		"commissionPercentage": "10.0000",
	}, gestorToken)
	require.Equal(t, http.StatusCreated, w.Code)

	var created map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	vendorID := created["id"].(string)

	// Buscar o vendedor criado.
	w2 := doVendorReq(t, router, http.MethodGet, "/api/v1/vendors/"+vendorID, nil, gestorToken)
	assert.Equal(t, http.StatusOK, w2.Code)

	var fetched map[string]any
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &fetched))
	assert.Equal(t, vendorID, fetched["id"])
}

// TestVendorInteg_VendedorCannotSeeOther — GET /vendors/{outro-id} com Vendedor → 403 (SC-005).
func TestVendorInteg_VendedorCannotSeeOther(t *testing.T) {
	router, _, issueToken := setupVendorInteg(t)

	// Criar um vendedor como Gestor.
	gestorToken := issueToken("gestor", "")
	w := doVendorReq(t, router, http.MethodPost, "/api/v1/vendors", map[string]string{
		"name":                 "Outro Vendedor",
		"email":                "integ-vendor-other@test.com",
		"commissionPercentage": "5.0000",
	}, gestorToken)
	require.Equal(t, http.StatusCreated, w.Code)

	var created map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	otherVendorID := created["id"].(string)

	// Tentar acessar como Vendedor com ID diferente.
	vendedorToken := issueToken("vendedor", "different-vendor-id")
	w2 := doVendorReq(t, router, http.MethodGet, "/api/v1/vendors/"+otherVendorID, nil, vendedorToken)
	assert.Equal(t, http.StatusForbidden, w2.Code, "SC-005: vendedor não pode acessar dados de outro")
}

// TestVendorInteg_Delete_LGPD — DELETE /vendors/{id} → 204 + anonimização.
func TestVendorInteg_Delete_LGPD(t *testing.T) {
	router, pool, issueToken := setupVendorInteg(t)
	gestorToken := issueToken("gestor", "")

	// Criar vendedor.
	w := doVendorReq(t, router, http.MethodPost, "/api/v1/vendors", map[string]string{
		"name":                 "Deletar LGPD",
		"email":                "integ-vendor-delete-lgpd@test.com",
		"commissionPercentage": "5.0000",
	}, gestorToken)
	require.Equal(t, http.StatusCreated, w.Code)

	var created map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	vendorID := created["id"].(string)

	// Confirmar anonimização.
	confirm := true
	w2 := doVendorReq(t, router, http.MethodDelete, "/api/v1/vendors/"+vendorID,
		map[string]any{"confirm": confirm}, gestorToken)
	assert.Equal(t, http.StatusNoContent, w2.Code, w2.Body.String())

	// Verificar no banco que PII foi anonimizada.
	ctx := context.Background()
	var name, email string
	var anonymizedAt *time.Time
	err := pool.QueryRow(ctx, `SELECT name, email, anonymized_at FROM vendors WHERE id = $1`, vendorID).
		Scan(&name, &email, &anonymizedAt)
	require.NoError(t, err)
	assert.Contains(t, name, "REMOVED_", "name deve conter REMOVED_")
	assert.Contains(t, email, "anon.invalid", "email deve conter anon.invalid")
	assert.NotNil(t, anonymizedAt, "anonymized_at deve estar preenchido")

	// Verificar registro no audit_trail.
	var count int
	pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM audit_trail
		WHERE entity_type = 'vendor_anonymization' AND entity_id = $1
	`, vendorID).Scan(&count)
	assert.Equal(t, 1, count, "deve ter 1 registro no audit_trail")
}

// TestVendorInteg_CreateInvalidPercentage — POST /vendors com percentual inválido → 400.
func TestVendorInteg_CreateInvalidPercentage(t *testing.T) {
	router, _, issueToken := setupVendorInteg(t)
	token := issueToken("gestor", "")

	w := doVendorReq(t, router, http.MethodPost, "/api/v1/vendors", map[string]string{
		"name":                 "Invalido",
		"email":                "integ-vendor-invalid-pct@test.com",
		"commissionPercentage": "150.0000", // > 100
	}, token)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
