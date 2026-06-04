// Package middleware — middleware de logging HTTP estruturado.
// Task 10.1.2: log de method, path, status, latência, request_id.
// Task 10.1.3: PII não logada em nível INFO; headers Authorization omitidos.
// Constitution P-I: auditabilidade via slog JSON em produção.
package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// StructuredLogger é um middleware slog que loga cada request com campos estruturados.
// Campos logados: method, path, status, latency_ms, request_id.
// Campos NUNCA logados: Authorization header, body de /auth/login (PII/senha).
func StructuredLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrapper de ResponseWriter para capturar status code.
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		// Request ID injetado pelo chimiddleware.RequestID.
		requestID := middleware.GetReqID(r.Context())

		// Processar request.
		next.ServeHTTP(ww, r)

		// Calcular latência.
		latencyMs := time.Since(start).Milliseconds()

		// Logar com slog — campos seguros (sem PII, sem Authorization).
		// path pode conter IDs de entidade (UUIDs) — não PII direta.
		// 10.1.3: NÃO logar headers de Authorization nem body de qualquer rota.
		slog.Info("http_request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"latency_ms", latencyMs,
			"request_id", requestID,
			"remote_addr", r.RemoteAddr,
		)
	})
}
