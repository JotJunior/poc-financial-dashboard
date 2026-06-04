// Package middleware fornece middlewares HTTP para autenticação e RBAC.
//
// Constitution P-IV — RBAC deny-by-default: todo endpoint autenticado
// DEVE usar RequireAuth + RequireRole. Ausência de middleware = acesso
// explicitamente não-protegido (decisão consciente do handler).
package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"financial-dashboard/backend/internal/auth"
)

// ─── Context key ─────────────────────────────────────────────────────────────

type contextKey string

const claimsKey contextKey = "auth_claims"

// WithClaims injeta *auth.Claims no contexto da requisição.
func WithClaims(ctx context.Context, c *auth.Claims) context.Context {
	return context.WithValue(ctx, claimsKey, c)
}

// ClaimsFromContext extrai *auth.Claims do contexto.
// Retorna (nil, false) se ausente — caller deve tratar como não-autenticado.
func ClaimsFromContext(ctx context.Context) (*auth.Claims, bool) {
	c, ok := ctx.Value(claimsKey).(*auth.Claims)
	return c, ok && c != nil
}

// ─── Respostas de erro ────────────────────────────────────────────────────────

type errResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	RefreshURL string `json:"refreshUrl,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, body errResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// ─── RequireAuth ──────────────────────────────────────────────────────────────

// AuthVerifier abstrai a verificação de token para permitir injeção de blocklist.
// Se nil, RequireAuth usa auth.VerifyToken (sem blocklist).
type AuthVerifier func(r *http.Request, tokenString string) (*auth.Claims, error)

// RequireAuth retorna um middleware que valida o JWT do header Authorization.
// Injeta *auth.Claims no contexto via WithClaims para uso pelos handlers downstream.
//
// Respostas:
//   - 401 {"error":"token_expired","refreshUrl":"/api/v1/auth/refresh"} — token expirado
//   - 401 {"error":"token_revoked"} — token na blocklist
//   - 401 {"error":"unauthorized"} — demais erros de token
//
// Se verifier for nil, usa auth.VerifyToken diretamente (sem checagem de blocklist).
func RequireAuth(verifier AuthVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractBearerToken(r)
			if token == "" {
				writeJSON(w, http.StatusUnauthorized, errResponse{Error: "unauthorized"})
				return
			}

			var claims *auth.Claims
			var err error

			if verifier != nil {
				claims, err = verifier(r, token)
			} else {
				claims, err = auth.VerifyToken(token)
			}

			if err != nil {
				switch {
				case errors.Is(err, auth.ErrTokenExpired):
					writeJSON(w, http.StatusUnauthorized, errResponse{
						Error:      "token_expired",
						Message:    "Token expirado",
						RefreshURL: "/api/v1/auth/refresh",
					})
				case errors.Is(err, auth.ErrTokenRevoked):
					writeJSON(w, http.StatusUnauthorized, errResponse{
						Error:   "token_revoked",
						Message: "Token revogado",
					})
				default:
					writeJSON(w, http.StatusUnauthorized, errResponse{Error: "unauthorized"})
				}
				return
			}

			ctx := WithClaims(r.Context(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// extractBearerToken extrai o token do header "Authorization: Bearer <token>".
// Retorna "" se ausente ou malformado.
func extractBearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

// ─── RequireRole ──────────────────────────────────────────────────────────────

// RequireRole retorna um middleware que verifica que o papel do usuário
// autenticado está na lista de papéis permitidos.
//
// Constitution P-IV deny-by-default: se Claims ausente no contexto → 403.
// Resposta: 403 {"error":"forbidden"}
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromContext(r.Context())
			if !ok {
				writeJSON(w, http.StatusForbidden, errResponse{Error: "forbidden"})
				return
			}

			if _, permitted := allowed[claims.Role]; !permitted {
				writeJSON(w, http.StatusForbidden, errResponse{Error: "forbidden"})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ─── RequireVendorScope ───────────────────────────────────────────────────────

// RequireVendorScope verifica que o vendedor autenticado só acessa seus próprios recursos.
// Gestor e Financeiro são isentos — podem acessar qualquer vendedor.
//
// Compara claims.VendorID com chi.URLParam(r, "vendorId") (prioritário) ou
// chi.URLParam(r, "vendorID") como fallback.
//
// Resposta: 403 {"error":"forbidden","message":"Acesso restrito ao próprio vendedor"}
func RequireVendorScope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusForbidden, errResponse{Error: "forbidden"})
			return
		}

		// Gestor e Financeiro: isentos da verificação de escopo.
		if claims.Role == "gestor" || claims.Role == "financeiro" {
			next.ServeHTTP(w, r)
			return
		}

		// Para papel "vendedor": verificar que o vendor_id do token bate com o da rota.
		routeVendorID := chi.URLParam(r, "vendorId")
		if routeVendorID == "" {
			routeVendorID = chi.URLParam(r, "vendorID")
		}

		if routeVendorID == "" || claims.VendorID == "" || claims.VendorID != routeVendorID {
			writeJSON(w, http.StatusForbidden, errResponse{
				Error:   "forbidden",
				Message: "Acesso restrito ao próprio vendedor",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}
