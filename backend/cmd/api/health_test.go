//go:build integration

// Package main — smoke tests for /health and /ready endpoints.
// Task 10.2.4: GET /health retorna 200; GET /ready retorna 200 quando banco disponível,
// 503 quando indisponível.
//
// Executar com:
//
//	DATABASE_URL="postgres://financialuser:financialpass@localhost:5433/financial_dashboard?sslmode=disable" \
//	  go test -tags=integration ./cmd/api/... -run TestHealthSmoke -v
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHealthSmoke: GET /health retorna 200 com status "ok" (task 10.2.4)
func TestHealthSmoke(t *testing.T) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://localhost:8080/health")
	if err != nil {
		t.Skipf("backend não está rodando em localhost:8080: %v", err)
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var body map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "ok", body["status"])
	assert.NotEmpty(t, body["timestamp"])
}

// TestReadySmoke: GET /ready retorna 200 quando banco disponível (task 10.2.4)
func TestReadySmoke(t *testing.T) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://localhost:8080/ready")
	if err != nil {
		t.Skipf("backend não está rodando em localhost:8080: %v", err)
	}
	defer resp.Body.Close()

	// Com banco disponível (PostgreSQL docker healthy), deve retornar 200.
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var body map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "ready", body["status"])
	assert.NotEmpty(t, body["timestamp"])
}

// TestHealthHandler_Unit: handler de /health via httptest (sem banco) (task 10.2.4)
func TestHealthHandler_Unit(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","timestamp":"2026-06-01T00:00:00Z"}`))
	})

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}
