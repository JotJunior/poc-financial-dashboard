//go:build integration

// Testes de integração para task 2.4.6 — requer PostgreSQL em localhost:5433.
// Executar com: go test -tags=integration ./internal/http/...
// Ou via: make test-integration
//
// O teste insere um usuário de teste, executa os fluxos login/refresh/logout
// contra o banco real e verifica revogação de tokens na tabela token_blocklist.
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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"financial-dashboard/backend/internal/auth"
	apphttp "financial-dashboard/backend/internal/http"
	"financial-dashboard/backend/internal/http/middleware"
	"financial-dashboard/backend/internal/repository"
)

const testDBURL = "postgres://financialuser:financialpass@localhost:5433/financial_dashboard?sslmode=disable"
const testSecret = "test-integration-secret-32chars!!"

func setupIntegration(t *testing.T) (*chi.Mux, *pgxpool.Pool) {
	t.Helper()
	t.Setenv("JWT_SECRET", testSecret)

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDBURL)
	require.NoError(t, err, "conexão ao PostgreSQL de teste")
	t.Cleanup(func() { pool.Close() })

	// Inserir usuário de teste.
	hash, err := auth.HashPassword("integ-test-pass-123")
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role)
		VALUES ('ffffffff-ffff-ffff-ffff-000000000001', 'Teste Integração', 'integ@test.com', $1, 'gestor')
		ON CONFLICT (email) DO UPDATE SET password_hash = $1
	`, hash)
	require.NoError(t, err)

	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM token_blocklist WHERE jti LIKE 'integ-%' OR jti IN (
			SELECT bl.jti FROM token_blocklist bl WHERE bl.revoked_at > now() - interval '1 hour'
		)`)
		pool.Exec(context.Background(), `DELETE FROM users WHERE email = 'integ@test.com'`)
	})

	userRepo := repository.NewUserRepository(pool)
	userAdapter := apphttp.NewUserAdapter(userRepo)
	blocklist := auth.NewDBBlocklist(pool)
	rl := apphttp.NewRateLimiter(5, 60*time.Second)
	handler := apphttp.NewAuthHandler(userAdapter, blocklist, rl)

	authVerifier := func(r *http.Request, tokenString string) (*auth.Claims, error) {
		return auth.VerifyTokenWithBlocklist(r.Context(), tokenString, blocklist)
	}

	r := chi.NewRouter()
	r.Post("/api/v1/auth/login", handler.Login)
	r.Post("/api/v1/auth/refresh", handler.Refresh)
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireAuth(authVerifier))
		r.Post("/api/v1/auth/logout", handler.Logout)
	})

	return r, pool
}

func doLoginInteg(t *testing.T, router http.Handler, email, password string) *httptest.ResponseRecorder {
	t.Helper()
	body := map[string]string{"email": email, "password": password}
	return postJSON(t, router, "/api/v1/auth/login", body)
}

// TestIntegration_LoginRefreshLogout verifica o fluxo completo contra o DB real.
func TestIntegration_LoginRefreshLogout(t *testing.T) {
	router, pool := setupIntegration(t)
	ctx := context.Background()

	// 1. Login com credenciais corretas → 200 + tokens.
	w := doLoginInteg(t, router, "integ@test.com", "integ-test-pass-123")
	require.Equal(t, http.StatusOK, w.Code, "login deve retornar 200")

	var loginBody map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &loginBody))
	accessToken, ok := loginBody["access_token"].(string)
	require.True(t, ok && accessToken != "", "access_token deve estar no body")

	var refreshCookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == "refresh_token" {
			refreshCookie = c
		}
	}
	require.NotNil(t, refreshCookie, "refresh_token deve estar no cookie httpOnly")
	assert.True(t, refreshCookie.HttpOnly)

	// 2. Refresh com token válido → novo access_token.
	w2 := postJSON(t, router, "/api/v1/auth/refresh", nil, refreshCookie)
	require.Equal(t, http.StatusOK, w2.Code, "refresh deve retornar 200")

	var refreshBody map[string]any
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &refreshBody))
	newAccessToken, ok := refreshBody["access_token"].(string)
	require.True(t, ok && newAccessToken != "")
	assert.NotEqual(t, accessToken, newAccessToken, "novo access_token deve ser diferente")

	// Novo refresh cookie (rotação).
	var newRefreshCookie *http.Cookie
	for _, c := range w2.Result().Cookies() {
		if c.Name == "refresh_token" {
			newRefreshCookie = c
		}
	}
	require.NotNil(t, newRefreshCookie)

	// 3. Token antigo não deve ser aceito novamente (revogado por rotação).
	w3 := postJSON(t, router, "/api/v1/auth/refresh", nil, refreshCookie)
	assert.Equal(t, http.StatusUnauthorized, w3.Code, "refresh token rotacionado deve ser rejeitado")

	// 4. Logout com novo access_token → 204 + token revogado no DB.
	w4 := postWithBearer(t, router, "/api/v1/auth/logout", newAccessToken, newRefreshCookie)
	assert.Equal(t, http.StatusNoContent, w4.Code, "logout deve retornar 204")

	// Verificar revogação no banco.
	claims, err := auth.VerifyToken(newAccessToken)
	require.NoError(t, err)
	var revoked bool
	err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM token_blocklist WHERE jti = $1)`, claims.ID).Scan(&revoked)
	require.NoError(t, err)
	assert.True(t, revoked, "JTI do access_token deve estar na tabela token_blocklist")
}

// TestIntegration_LoginWrongPassword → 401.
func TestIntegration_LoginWrongPassword(t *testing.T) {
	router, _ := setupIntegration(t)
	w := doLoginInteg(t, router, "integ@test.com", "senha-errada")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestIntegration_RateLimit5x → 429 na 6ª tentativa por IP.
func TestIntegration_RateLimit5x(t *testing.T) {
	t.Setenv("JWT_SECRET", testSecret)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDBURL)
	require.NoError(t, err)
	defer pool.Close()

	userRepo := repository.NewUserRepository(pool)
	userAdapter := apphttp.NewUserAdapter(userRepo)
	blocklist := auth.NewDBBlocklist(pool)

	// Rate limiter apertado: 5/60s.
	rl := apphttp.NewRateLimiter(5, 60*time.Second)
	handler := apphttp.NewAuthHandler(userAdapter, blocklist, rl)

	r := chi.NewRouter()
	r.Post("/api/v1/auth/login", handler.Login)

	body := map[string]string{"email": "qualquer@test.com", "password": "errada"}
	for i := 0; i < 5; i++ {
		w := postJSON(t, r, "/api/v1/auth/login", body)
		// Deve ser 401 (usuário não existe), não 429.
		assert.NotEqual(t, http.StatusTooManyRequests, w.Code, "tentativa %d não deve ser rate-limited", i+1)
	}
	// 6ª → rate limited.
	w := postJSON(t, r, "/api/v1/auth/login", body)
	assert.Equal(t, http.StatusTooManyRequests, w.Code, "6ª tentativa deve retornar 429")
}
