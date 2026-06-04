// Package http — handlers de autenticação: login, refresh, logout.
// Task 2.4: POST /auth/login, /auth/refresh, /auth/logout.
// CHK026: refresh_token em httpOnly cookie; access_token em JSON body.
// CHK: rate-limit em /auth/login — max 5 tentativas/IP/60s (OWASP finding medium).
// Constitution P-IV RBAC deny-by-default; P-I auditabilidade.
package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"financial-dashboard/backend/internal/auth"
	"financial-dashboard/backend/internal/http/middleware"
)

// ─── Interfaces ───────────────────────────────────────────────────────────────

// AuthUserRow contém os campos de usuário necessários para o fluxo de auth.
// Espelhado do repository.UserRow — definido aqui para desacoplar o handler
// do pacote repository e facilitar injeção em testes sem dependência de DB.
type AuthUserRow struct {
	ID           string
	PasswordHash string
	Role         string
	VendorID     *string // nil quando role != "vendedor"
}

// AuthUserFinder localiza um usuário por email ou ID — implementado por *UserAdapter.
type AuthUserFinder interface {
	FindByEmailAuth(ctx context.Context, email string) (AuthUserRow, error)
	FindByIDAuth(ctx context.Context, id string) (AuthUserRow, error)
}

// AuthBlocklist revoga e verifica JTIs — implementado por auth.DBBlocklist.
type AuthBlocklist interface {
	Revoke(ctx context.Context, jti string, expiresAt time.Time, reason string) error
	IsRevoked(ctx context.Context, jti string) (bool, error)
}

// ─── AuthHandler ──────────────────────────────────────────────────────────────

// AuthHandler agrupa os handlers de /auth/*.
type AuthHandler struct {
	users       AuthUserFinder
	blocklist   AuthBlocklist
	rateLimiter *RateLimiter
}

// NewAuthHandler cria um AuthHandler com as dependências fornecidas.
// rateLimiter pode ser nil — nesse caso /login não tem rate-limit (apenas para testes).
func NewAuthHandler(users AuthUserFinder, bl AuthBlocklist, rl *RateLimiter) *AuthHandler {
	return &AuthHandler{
		users:       users,
		blocklist:   bl,
		rateLimiter: rl,
	}
}

// ─── DTOs ─────────────────────────────────────────────────────────────────────

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"` // segundos
}

type authErrJSON struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func writeAuthJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

const refreshCookieName = "refresh_token"
const refreshTokenTTL = 7 * 24 * time.Hour

// setRefreshCookie grava o refresh_token em httpOnly cookie (CHK026).
// Secure=true: em dev local sem TLS o browser não envia o cookie — comportamento correto
// (não deve trafegar em HTTP). Para testes de integração usar httptest com mock de cookie.
func setRefreshCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    token,
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(refreshTokenTTL.Seconds()),
	})
}

// clearRefreshCookie apaga o cookie de refresh no logout.
func clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}

func vendorIDStr(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

// ─── Login ────────────────────────────────────────────────────────────────────

// Login — POST /api/v1/auth/login
// Body: {"email":"...","password":"..."}
// 200: {"access_token":"...","token_type":"Bearer","expires_in":900}
//   + Set-Cookie: refresh_token=...; HttpOnly; Secure; SameSite=Strict
// 401: {"error":"invalid_credentials"}
// 429: {"error":"rate_limit_exceeded","message":"..."}
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	// Rate limit por IP — max 5 tentativas em 60s (OWASP CHK medium).
	if h.rateLimiter != nil {
		if !h.rateLimiter.Allow(extractIP(r)) {
			writeAuthJSON(w, http.StatusTooManyRequests, authErrJSON{
				Error:   "rate_limit_exceeded",
				Message: "Muitas tentativas de login. Tente novamente em 60 segundos.",
			})
			return
		}
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAuthJSON(w, http.StatusBadRequest, authErrJSON{Error: "invalid_request"})
		return
	}
	if req.Email == "" || req.Password == "" {
		writeAuthJSON(w, http.StatusUnauthorized, authErrJSON{Error: "invalid_credentials"})
		return
	}

	user, err := h.users.FindByEmailAuth(r.Context(), req.Email)
	if err != nil {
		// Não vazar se email existe — resposta genérica em qualquer erro.
		writeAuthJSON(w, http.StatusUnauthorized, authErrJSON{Error: "invalid_credentials"})
		return
	}

	if !auth.VerifyPassword(req.Password, user.PasswordHash) {
		writeAuthJSON(w, http.StatusUnauthorized, authErrJSON{Error: "invalid_credentials"})
		return
	}

	accessToken, err := auth.IssueAccessToken(user.ID, user.Role, vendorIDStr(user.VendorID))
	if err != nil {
		slog.Error("auth: emitir access token no login", "err", err)
		writeAuthJSON(w, http.StatusInternalServerError, authErrJSON{Error: "internal_error"})
		return
	}

	refreshToken, err := auth.IssueRefreshToken(user.ID)
	if err != nil {
		slog.Error("auth: emitir refresh token no login", "err", err)
		writeAuthJSON(w, http.StatusInternalServerError, authErrJSON{Error: "internal_error"})
		return
	}

	// Refresh SOMENTE no httpOnly cookie — nunca no body (CHK026).
	setRefreshCookie(w, refreshToken)

	writeAuthJSON(w, http.StatusOK, tokenResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   900, // 15 min em segundos
	})
}

// ─── Refresh ──────────────────────────────────────────────────────────────────

// Refresh — POST /api/v1/auth/refresh
// Cookie: refresh_token=<jwt>
// 200: {"access_token":"...","token_type":"Bearer","expires_in":900}
//   + Set-Cookie: refresh_token=<novo_jwt>; HttpOnly; Secure
// 401: {"error":"invalid_token"}
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(refreshCookieName)
	if err != nil {
		writeAuthJSON(w, http.StatusUnauthorized, authErrJSON{
			Error:   "invalid_token",
			Message: "refresh_token ausente",
		})
		return
	}

	claims, err := auth.VerifyToken(cookie.Value)
	if err != nil {
		writeAuthJSON(w, http.StatusUnauthorized, authErrJSON{Error: "invalid_token"})
		return
	}

	// Verificar se o refresh token foi revogado.
	revoked, err := h.blocklist.IsRevoked(r.Context(), claims.ID)
	if err != nil {
		slog.Error("auth: verificar blocklist no refresh", "err", err)
		writeAuthJSON(w, http.StatusInternalServerError, authErrJSON{Error: "internal_error"})
		return
	}
	if revoked {
		writeAuthJSON(w, http.StatusUnauthorized, authErrJSON{
			Error:   "invalid_token",
			Message: "token revogado",
		})
		return
	}

	// Revogar o refresh token atual (rotação obrigatória — previne reuso).
	var expTime time.Time
	if claims.ExpiresAt != nil {
		expTime = claims.ExpiresAt.Time
	}
	if err := h.blocklist.Revoke(r.Context(), claims.ID, expTime, "refresh_rotation"); err != nil {
		slog.Error("auth: revogar refresh token antigo no refresh", "err", err)
		writeAuthJSON(w, http.StatusInternalServerError, authErrJSON{Error: "internal_error"})
		return
	}

	// Buscar usuário para reconstruir role/vendor_id no novo access token.
	// O refresh token carrega apenas sub=userID por design (task 2.1.3).
	user, err := h.users.FindByIDAuth(r.Context(), claims.Subject)
	if err != nil {
		slog.Error("auth: buscar usuário no refresh", "err", err, "userID", claims.Subject)
		writeAuthJSON(w, http.StatusUnauthorized, authErrJSON{Error: "invalid_token"})
		return
	}

	newAccessToken, err := auth.IssueAccessToken(user.ID, user.Role, vendorIDStr(user.VendorID))
	if err != nil {
		slog.Error("auth: emitir novo access token no refresh", "err", err)
		writeAuthJSON(w, http.StatusInternalServerError, authErrJSON{Error: "internal_error"})
		return
	}

	newRefreshToken, err := auth.IssueRefreshToken(user.ID)
	if err != nil {
		slog.Error("auth: emitir novo refresh token no refresh", "err", err)
		writeAuthJSON(w, http.StatusInternalServerError, authErrJSON{Error: "internal_error"})
		return
	}

	setRefreshCookie(w, newRefreshToken)

	writeAuthJSON(w, http.StatusOK, tokenResponse{
		AccessToken: newAccessToken,
		TokenType:   "Bearer",
		ExpiresIn:   900,
	})
}

// ─── Logout ───────────────────────────────────────────────────────────────────

// Logout — POST /api/v1/auth/logout
// Header: Authorization: Bearer <access_token>  (via RequireAuth middleware)
// Cookie: refresh_token=<jwt> (opcional — revogado se presente)
// 204: No Content
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// Revogar access token (claims injetadas pelo RequireAuth middleware).
	if claims, ok := middleware.ClaimsFromContext(r.Context()); ok && claims.ID != "" {
		var expTime time.Time
		if claims.ExpiresAt != nil {
			expTime = claims.ExpiresAt.Time
		}
		if err := h.blocklist.Revoke(r.Context(), claims.ID, expTime, "logout"); err != nil {
			slog.Error("auth: revogar access token no logout", "err", err)
			// Não interromper o logout — objetivo é limpar a sessão do cliente.
		}
	}

	// Revogar refresh token se presente no cookie.
	if cookie, err := r.Cookie(refreshCookieName); err == nil {
		if refreshClaims, err := auth.VerifyToken(cookie.Value); err == nil && refreshClaims.ID != "" {
			var expTime time.Time
			if refreshClaims.ExpiresAt != nil {
				expTime = refreshClaims.ExpiresAt.Time
			}
			if err := h.blocklist.Revoke(r.Context(), refreshClaims.ID, expTime, "logout"); err != nil {
				slog.Error("auth: revogar refresh token no logout", "err", err)
			}
		}
	}

	clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}
