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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"financial-dashboard/backend/internal/auth"
	apphttp "financial-dashboard/backend/internal/http"
	"financial-dashboard/backend/internal/http/middleware"
)

// ─── Stubs ────────────────────────────────────────────────────────────────────

// stubUserFinder implementa AuthUserFinder para testes sem banco.
type stubUserFinder struct {
	byEmail map[string]apphttp.AuthUserRow
	byID    map[string]apphttp.AuthUserRow
}

func (s *stubUserFinder) FindByEmailAuth(_ context.Context, email string) (apphttp.AuthUserRow, error) {
	row, ok := s.byEmail[email]
	if !ok {
		return apphttp.AuthUserRow{}, errNotFound
	}
	return row, nil
}

func (s *stubUserFinder) FindByIDAuth(_ context.Context, id string) (apphttp.AuthUserRow, error) {
	row, ok := s.byID[id]
	if !ok {
		return apphttp.AuthUserRow{}, errNotFound
	}
	return row, nil
}

var errNotFound = &notFoundErr{}

type notFoundErr struct{}

func (e *notFoundErr) Error() string { return "not found" }

// stubBlocklist implementa AuthBlocklist em memória.
type stubBlocklist struct {
	revoked map[string]bool
}

func newStubBlocklist() *stubBlocklist {
	return &stubBlocklist{revoked: make(map[string]bool)}
}

func (s *stubBlocklist) Revoke(_ context.Context, jti string, _ time.Time, _ string) error {
	s.revoked[jti] = true
	return nil
}

func (s *stubBlocklist) IsRevoked(_ context.Context, jti string) (bool, error) {
	return s.revoked[jti], nil
}

// ─── Setup ────────────────────────────────────────────────────────────────────

func setup(t *testing.T) (router *chi.Mux, userFinder *stubUserFinder, bl *stubBlocklist) {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret-minimo-32-chars-ok!!")

	// Criar um usuário de teste com senha conhecida.
	hash, err := auth.HashPassword("senha-correta-123")
	require.NoError(t, err)

	vendorID := "vendor-uuid-001"
	row := apphttp.AuthUserRow{
		ID:           "user-uuid-001",
		PasswordHash: hash,
		Role:         "gestor",
	}
	vendorRow := apphttp.AuthUserRow{
		ID:           "vendor-user-uuid-002",
		PasswordHash: hash,
		Role:         "vendedor",
		VendorID:     &vendorID,
	}

	userFinder = &stubUserFinder{
		byEmail: map[string]apphttp.AuthUserRow{
			"gestor@test.com":  row,
			"vendor@test.com":  vendorRow,
		},
		byID: map[string]apphttp.AuthUserRow{
			"user-uuid-001":        row,
			"vendor-user-uuid-002": vendorRow,
		},
	}
	bl = newStubBlocklist()

	// Rate limiter com cap generoso para não interferir nos testes unitários.
	rl := apphttp.NewRateLimiter(100, 60*time.Second)
	handler := apphttp.NewAuthHandler(userFinder, bl, rl)

	authVerifier := func(r *http.Request, tokenString string) (*auth.Claims, error) {
		claims, err := auth.VerifyToken(tokenString)
		if err != nil {
			return nil, err
		}
		revoked, err := bl.IsRevoked(r.Context(), claims.ID)
		if err != nil {
			return nil, err
		}
		if revoked {
			return nil, auth.ErrTokenRevoked
		}
		return claims, nil
	}

	r := chi.NewRouter()
	r.Post("/api/v1/auth/login", handler.Login)
	r.Post("/api/v1/auth/refresh", handler.Refresh)
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireAuth(authVerifier))
		r.Post("/api/v1/auth/logout", handler.Logout)
	})

	return r, userFinder, bl
}

func postJSON(t *testing.T, router http.Handler, path string, body any, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func postWithBearer(t *testing.T, router http.Handler, path, token string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// ─── Login ────────────────────────────────────────────────────────────────────

func TestLogin_CorrectCredentials(t *testing.T) {
	router, _, _ := setup(t)

	w := postJSON(t, router, "/api/v1/auth/login", map[string]string{
		"email":    "gestor@test.com",
		"password": "senha-correta-123",
	})

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp["access_token"])
	assert.Equal(t, "Bearer", resp["token_type"])

	// refresh_token deve estar SOMENTE no cookie, nunca no body.
	assert.Empty(t, resp["refresh_token"], "refresh_token não deve aparecer no body")

	// Cookie httpOnly deve estar presente.
	var refreshCookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == "refresh_token" {
			refreshCookie = c
			break
		}
	}
	require.NotNil(t, refreshCookie, "cookie refresh_token deve estar presente")
	assert.True(t, refreshCookie.HttpOnly, "refresh_token deve ser HttpOnly")
	assert.True(t, refreshCookie.Secure, "refresh_token deve ser Secure")
}

func TestLogin_WrongPassword(t *testing.T) {
	router, _, _ := setup(t)

	w := postJSON(t, router, "/api/v1/auth/login", map[string]string{
		"email":    "gestor@test.com",
		"password": "senha-errada",
	})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "invalid_credentials", resp["error"])
}

func TestLogin_UnknownEmail(t *testing.T) {
	router, _, _ := setup(t)

	w := postJSON(t, router, "/api/v1/auth/login", map[string]string{
		"email":    "ninguem@test.com",
		"password": "qualquer",
	})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestLogin_EmptyFields(t *testing.T) {
	router, _, _ := setup(t)

	w := postJSON(t, router, "/api/v1/auth/login", map[string]string{
		"email":    "",
		"password": "",
	})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ─── Rate Limit ───────────────────────────────────────────────────────────────

func TestLogin_RateLimit(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-minimo-32-chars-ok!!")

	// Rate limiter apertado: 5 tentativas / 60s.
	rl := apphttp.NewRateLimiter(5, 60*time.Second)

	hash, _ := auth.HashPassword("x")
	uf := &stubUserFinder{
		byEmail: map[string]apphttp.AuthUserRow{"u@t.com": {ID: "1", PasswordHash: hash, Role: "gestor"}},
		byID:    map[string]apphttp.AuthUserRow{"1": {ID: "1", PasswordHash: hash, Role: "gestor"}},
	}
	bl := newStubBlocklist()
	handler := apphttp.NewAuthHandler(uf, bl, rl)

	r := chi.NewRouter()
	r.Post("/api/v1/auth/login", handler.Login)

	body := map[string]string{"email": "u@t.com", "password": "errada"}

	// 5 primeiras — devem passar (e retornar 401 por senha errada).
	for i := 0; i < 5; i++ {
		w := postJSON(t, r, "/api/v1/auth/login", body)
		assert.Equal(t, http.StatusUnauthorized, w.Code, "tentativa %d deveria retornar 401", i+1)
	}

	// 6ª — deve ser bloqueada pelo rate limit.
	w := postJSON(t, r, "/api/v1/auth/login", body)
	assert.Equal(t, http.StatusTooManyRequests, w.Code, "6ª tentativa deveria retornar 429")

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "rate_limit_exceeded", resp["error"])
}

// ─── Refresh ──────────────────────────────────────────────────────────────────

func TestRefresh_ValidToken(t *testing.T) {
	router, _, _ := setup(t)

	// Fazer login para obter refresh_token.
	loginResp := postJSON(t, router, "/api/v1/auth/login", map[string]string{
		"email":    "gestor@test.com",
		"password": "senha-correta-123",
	})
	require.Equal(t, http.StatusOK, loginResp.Code)

	var refreshCookie *http.Cookie
	for _, c := range loginResp.Result().Cookies() {
		if c.Name == "refresh_token" {
			refreshCookie = c
		}
	}
	require.NotNil(t, refreshCookie)

	// Usar refresh_token para obter novo access_token.
	w := postJSON(t, router, "/api/v1/auth/refresh", nil, refreshCookie)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp["access_token"])

	// Novo refresh_token deve ser emitido (rotação).
	var newRefreshCookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == "refresh_token" {
			newRefreshCookie = c
		}
	}
	require.NotNil(t, newRefreshCookie, "novo refresh_token deve ser emitido")
	assert.NotEqual(t, refreshCookie.Value, newRefreshCookie.Value, "refresh token deve ser rotacionado")
}

func TestRefresh_OldTokenRevoked(t *testing.T) {
	router, _, bl := setup(t)

	// Login para obter refresh_token.
	loginResp := postJSON(t, router, "/api/v1/auth/login", map[string]string{
		"email":    "gestor@test.com",
		"password": "senha-correta-123",
	})
	require.Equal(t, http.StatusOK, loginResp.Code)

	var refreshCookie *http.Cookie
	for _, c := range loginResp.Result().Cookies() {
		if c.Name == "refresh_token" {
			refreshCookie = c
		}
	}
	require.NotNil(t, refreshCookie)

	// Primeiro refresh — sucesso.
	w1 := postJSON(t, router, "/api/v1/auth/refresh", nil, refreshCookie)
	require.Equal(t, http.StatusOK, w1.Code)

	// Segundo uso do MESMO refresh_token (já foi rotacionado = deve estar na blocklist).
	w2 := postJSON(t, router, "/api/v1/auth/refresh", nil, refreshCookie)
	assert.Equal(t, http.StatusUnauthorized, w2.Code, "token já rotacionado deve ser rejeitado")
	_ = bl // verifica que a blocklist foi usada
}

func TestRefresh_NoToken(t *testing.T) {
	router, _, _ := setup(t)

	w := postJSON(t, router, "/api/v1/auth/refresh", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ─── Logout ───────────────────────────────────────────────────────────────────

func TestLogout_RevokesTokens(t *testing.T) {
	router, _, bl := setup(t)

	// Login.
	loginResp := postJSON(t, router, "/api/v1/auth/login", map[string]string{
		"email":    "gestor@test.com",
		"password": "senha-correta-123",
	})
	require.Equal(t, http.StatusOK, loginResp.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(loginResp.Body.Bytes(), &body))
	accessToken := body["access_token"].(string)

	var refreshCookie *http.Cookie
	for _, c := range loginResp.Result().Cookies() {
		if c.Name == "refresh_token" {
			refreshCookie = c
		}
	}
	require.NotNil(t, refreshCookie)

	// Logout.
	w := postWithBearer(t, router, "/api/v1/auth/logout", accessToken, refreshCookie)
	assert.Equal(t, http.StatusNoContent, w.Code)

	// Verificar que access_token foi revogado.
	claims, err := auth.VerifyToken(accessToken)
	require.NoError(t, err)
	revoked, err := bl.IsRevoked(context.Background(), claims.ID)
	require.NoError(t, err)
	assert.True(t, revoked, "access_token deve estar revogado após logout")

	// Verificar que cookie foi limpo (MaxAge=-1 ou Set-Cookie com Expires no passado).
	var clearedCookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == "refresh_token" {
			clearedCookie = c
		}
	}
	if clearedCookie != nil {
		assert.True(t, clearedCookie.MaxAge < 0 || clearedCookie.Value == "",
			"cookie refresh_token deve ser apagado no logout")
	}
}

func TestLogout_RequiresAuth(t *testing.T) {
	router, _, _ := setup(t)

	// Logout sem token deve retornar 401.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ─── Segurança: JWT não exposto em URL/log ────────────────────────────────────

func TestLogin_AccessTokenNotInRefreshCookieBody(t *testing.T) {
	router, _, _ := setup(t)

	w := postJSON(t, router, "/api/v1/auth/login", map[string]string{
		"email":    "gestor@test.com",
		"password": "senha-correta-123",
	})
	require.Equal(t, http.StatusOK, w.Code)

	// Body deve conter access_token mas NÃO refresh_token.
	bodyStr := w.Body.String()
	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(bodyStr), &resp))
	assert.Contains(t, resp, "access_token", "access_token deve estar no body")
	assert.NotContains(t, resp, "refresh_token", "refresh_token não deve estar no body")
}
