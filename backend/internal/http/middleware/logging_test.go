// Package middleware — testes do middleware de logging estruturado.
// Task 10.1.4: verificar ausência de `password` e `password_hash` no log de /auth/login.
// Task 10.1.3: PII não logada em INFO; Authorization omitido.
package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureLogger cria um slog.Logger que escreve em um bytes.Buffer (para testes).
func captureLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// TestStructuredLogger_NoPIIInLoginLog (task 10.1.4):
// Verifica que o log do request POST /auth/login NÃO contém "password", "password_hash"
// nem o token Authorization.
func TestStructuredLogger_NoPIIInLoginLog(t *testing.T) {
	var logBuf bytes.Buffer
	oldDefault := slog.Default()
	slog.SetDefault(captureLogger(&logBuf))
	defer slog.SetDefault(oldDefault)

	// Handler que simula /auth/login — produz 200 OK.
	loginHandler := StructuredLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"access_token":"jwt-token-value"}`))
	}))

	// Construir request com body de login e header Authorization.
	body := `{"email":"gestor@test.com","password":"super-secret-123","password_hash":"should-not-appear"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiJ9.sensitive")

	rec := httptest.NewRecorder()
	loginHandler.ServeHTTP(rec, req)

	// Verificar que o handler respondeu 200.
	assert.Equal(t, http.StatusOK, rec.Code)

	// Verificar que o log NÃO contém campos sensíveis.
	logOutput := logBuf.String()

	// 10.1.4: senha nunca deve aparecer no log
	assert.NotContains(t, logOutput, "super-secret-123",
		"senha do usuário não deve aparecer no log")
	assert.NotContains(t, logOutput, "password_hash",
		"password_hash não deve aparecer no log")
	assert.NotContains(t, logOutput, "password",
		"campo password não deve aparecer no log")

	// 10.1.3: token de Authorization não deve aparecer no log
	assert.NotContains(t, logOutput, "eyJhbGciOiJIUzI1NiJ9.sensitive",
		"token Authorization não deve aparecer no log")
	assert.NotContains(t, logOutput, "Authorization",
		"header Authorization não deve aparecer no log (não é logado)")

	// Verificar que campos seguros ESTÃO presentes no log.
	require.NotEmpty(t, logOutput, "log deve ter ao menos uma entrada")

	var logEntry map[string]any
	// Parsear primeira linha do log (JSON).
	firstLine := strings.SplitN(logOutput, "\n", 2)[0]
	require.NoError(t, json.Unmarshal([]byte(firstLine), &logEntry))

	assert.Equal(t, "POST", logEntry["method"])
	assert.Equal(t, "/api/v1/auth/login", logEntry["path"])
	assert.Equal(t, float64(200), logEntry["status"])
	assert.Contains(t, logEntry, "latency_ms")
}

// TestStructuredLogger_SafeFields: verifica que campos seguros são logados.
func TestStructuredLogger_SafeFields(t *testing.T) {
	var logBuf bytes.Buffer
	oldDefault := slog.Default()
	slog.SetDefault(captureLogger(&logBuf))
	defer slog.SetDefault(oldDefault)

	handler := StructuredLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	req := httptest.NewRequest("GET", "/api/v1/vendors", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	logOutput := logBuf.String()
	require.NotEmpty(t, logOutput)

	var entry map[string]any
	firstLine := strings.SplitN(logOutput, "\n", 2)[0]
	require.NoError(t, json.Unmarshal([]byte(firstLine), &entry))

	// Campos obrigatórios (10.1.2)
	assert.Equal(t, "GET", entry["method"])
	assert.Equal(t, "/api/v1/vendors", entry["path"])
	assert.Equal(t, float64(404), entry["status"])
	assert.Contains(t, entry, "latency_ms")
	assert.Contains(t, entry, "request_id") // pode ser empty string se não injetado
}
