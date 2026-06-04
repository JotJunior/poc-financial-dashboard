package middleware_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"financial-dashboard/backend/internal/auth"
	"financial-dashboard/backend/internal/http/middleware"
)

// ─── Helpers ──────────────────────────────────────────────────────────────────

func setSecret(t *testing.T) {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret-minimo-32-chars-ok!!")
}

func issueToken(t *testing.T, userID, role, vendorID string) string {
	t.Helper()
	tok, err := auth.IssueAccessToken(userID, role, vendorID)
	require.NoError(t, err)
	return tok
}

func expiredToken(t *testing.T, secret string) string {
	t.Helper()
	claims := auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-exp",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			ID:        "jti-expired",
		},
		Role: "gestor",
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString([]byte(secret))
	require.NoError(t, err)
	return s
}

func newRequest(t *testing.T, token string) *http.Request {
	t.Helper()
	r := httptest.NewRequest("GET", "/", nil)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	return r
}

func okHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func body(t *testing.T, rec *httptest.ResponseRecorder) map[string]string {
	t.Helper()
	var m map[string]string
	err := json.Unmarshal(rec.Body.Bytes(), &m)
	require.NoError(t, err)
	return m
}

// ─── RequireAuth tests ────────────────────────────────────────────────────────

// TestRequireAuth_NoToken: request sem token → 401.
func TestRequireAuth_NoToken(t *testing.T) {
	setSecret(t)
	mw := middleware.RequireAuth(nil)
	rec := httptest.NewRecorder()
	mw(http.HandlerFunc(okHandler)).ServeHTTP(rec, newRequest(t, ""))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, "unauthorized", body(t, rec)["error"])
}

// TestRequireAuth_InvalidToken: token inválido → 401.
func TestRequireAuth_InvalidToken(t *testing.T) {
	setSecret(t)
	mw := middleware.RequireAuth(nil)
	rec := httptest.NewRecorder()
	mw(http.HandlerFunc(okHandler)).ServeHTTP(rec, newRequest(t, "token.invalido.aqui"))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, "unauthorized", body(t, rec)["error"])
}

// TestRequireAuth_ExpiredToken: token expirado → 401 com body token_expired + refreshUrl.
func TestRequireAuth_ExpiredToken(t *testing.T) {
	secret := "test-secret-minimo-32-chars-ok!!"
	t.Setenv("JWT_SECRET", secret)

	tok := expiredToken(t, secret)
	mw := middleware.RequireAuth(nil)
	rec := httptest.NewRecorder()
	mw(http.HandlerFunc(okHandler)).ServeHTTP(rec, newRequest(t, tok))

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	b := body(t, rec)
	assert.Equal(t, "token_expired", b["error"])
	assert.Equal(t, "/api/v1/auth/refresh", b["refreshUrl"])
}

// TestRequireAuth_ValidToken: token válido → próximo handler chamado com claims no contexto.
func TestRequireAuth_ValidToken(t *testing.T) {
	setSecret(t)
	tok := issueToken(t, "user-123", "gestor", "")
	mw := middleware.RequireAuth(nil)

	var gotClaims *auth.Claims
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, ok := middleware.ClaimsFromContext(r.Context())
		if ok {
			gotClaims = c
		}
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	mw(handler).ServeHTTP(rec, newRequest(t, tok))

	assert.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, gotClaims, "claims deve estar no contexto após RequireAuth")
	assert.Equal(t, "user-123", gotClaims.Subject)
	assert.Equal(t, "gestor", gotClaims.Role)
}

// TestRequireAuth_RevokedToken: verifier retorna ErrTokenRevoked → 401 token_revoked.
func TestRequireAuth_RevokedToken(t *testing.T) {
	setSecret(t)
	tok := issueToken(t, "user-rev", "vendedor", "v-001")

	revokedVerifier := middleware.AuthVerifier(func(_ *http.Request, _ string) (*auth.Claims, error) {
		return nil, auth.ErrTokenRevoked
	})

	mw := middleware.RequireAuth(revokedVerifier)
	rec := httptest.NewRecorder()
	mw(http.HandlerFunc(okHandler)).ServeHTTP(rec, newRequest(t, tok))

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, "token_revoked", body(t, rec)["error"])
}

// ─── RequireRole tests ────────────────────────────────────────────────────────

// TestRequireRole_WrongRole: papel errado → 403.
func TestRequireRole_WrongRole(t *testing.T) {
	setSecret(t)
	tok := issueToken(t, "user-v", "vendedor", "v-001")

	mw := middleware.RequireAuth(nil)
	roleMw := middleware.RequireRole("gestor")

	handler := http.HandlerFunc(okHandler)
	chain := mw(roleMw(handler))

	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, newRequest(t, tok))

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t, "forbidden", body(t, rec)["error"])
}

// TestRequireRole_CorrectRole: papel correto → handler chamado.
func TestRequireRole_CorrectRole(t *testing.T) {
	setSecret(t)
	tok := issueToken(t, "user-g", "gestor", "")

	mw := middleware.RequireAuth(nil)
	roleMw := middleware.RequireRole("gestor", "financeiro")

	chain := mw(roleMw(http.HandlerFunc(okHandler)))
	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, newRequest(t, tok))

	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestRequireRole_NoClaims: sem RequireAuth antes → 403 (deny-by-default).
func TestRequireRole_NoClaims(t *testing.T) {
	roleMw := middleware.RequireRole("gestor")
	rec := httptest.NewRecorder()
	roleMw(http.HandlerFunc(okHandler)).ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// ─── RequireVendorScope tests ─────────────────────────────────────────────────

func newChiRequest(t *testing.T, token, vendorID string) *http.Request {
	t.Helper()
	r := httptest.NewRequest("GET", "/api/v1/vendors/"+vendorID+"/orders", nil)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	// Injetar URL params via chi context.
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("vendorId", vendorID)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// TestRequireVendorScope_VendorAccessOwn: vendedor acessando próprio recurso → passa.
func TestRequireVendorScope_VendorAccessOwn(t *testing.T) {
	setSecret(t)
	tok := issueToken(t, "user-v", "vendedor", "v-001")

	// Cadeia: RequireAuth → RequireVendorScope → handler
	authMw := middleware.RequireAuth(nil)
	chain := authMw(middleware.RequireVendorScope(http.HandlerFunc(okHandler)))

	req := newChiRequest(t, tok, "v-001")
	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestRequireVendorScope_VendorAccessOther: vendedor acessando outro → 403.
func TestRequireVendorScope_VendorAccessOther(t *testing.T) {
	setSecret(t)
	tok := issueToken(t, "user-v", "vendedor", "v-001")

	authMw := middleware.RequireAuth(nil)
	chain := authMw(middleware.RequireVendorScope(http.HandlerFunc(okHandler)))

	// Tentando acessar v-002 mas token tem v-001.
	req := newChiRequest(t, tok, "v-002")
	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t, "forbidden", body(t, rec)["error"])
}

// TestRequireVendorScope_GestorExempt: gestor pode acessar qualquer vendedor.
func TestRequireVendorScope_GestorExempt(t *testing.T) {
	setSecret(t)
	tok := issueToken(t, "user-g", "gestor", "")

	authMw := middleware.RequireAuth(nil)
	chain := authMw(middleware.RequireVendorScope(http.HandlerFunc(okHandler)))

	req := newChiRequest(t, tok, "v-qualquer-999")
	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestRequireVendorScope_FinanceiroExempt: financeiro pode acessar qualquer vendedor.
func TestRequireVendorScope_FinanceiroExempt(t *testing.T) {
	setSecret(t)
	tok := issueToken(t, "user-f", "financeiro", "")

	authMw := middleware.RequireAuth(nil)
	chain := authMw(middleware.RequireVendorScope(http.HandlerFunc(okHandler)))

	req := newChiRequest(t, tok, "v-qualquer-888")
	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// ─── Context helpers ──────────────────────────────────────────────────────────

// TestClaimsFromContext_Empty: contexto sem claims retorna (nil, false).
func TestClaimsFromContext_Empty(t *testing.T) {
	c, ok := middleware.ClaimsFromContext(context.Background())
	assert.False(t, ok)
	assert.Nil(t, c)
}

// TestWithClaims_RoundTrip: WithClaims e ClaimsFromContext são inversos.
func TestWithClaims_RoundTrip(t *testing.T) {
	original := &auth.Claims{}
	original.Subject = "user-rt"
	original.Role = "financeiro"

	ctx := middleware.WithClaims(context.Background(), original)
	got, ok := middleware.ClaimsFromContext(ctx)
	require.True(t, ok)
	assert.Equal(t, "user-rt", got.Subject)
	assert.Equal(t, "financeiro", got.Role)
}
