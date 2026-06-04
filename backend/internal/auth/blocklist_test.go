//go:build integration

// Package auth — blocklist integration tests.
// Task 2.2.6 / CHK011
// Requer: DATABASE_URL com tabela token_blocklist criada (migration 010).
// Executar: DATABASE_URL="..." go test ./internal/auth/... -v -tags integration -run TestBlocklist
package auth

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupPool conecta ao banco de dados via DATABASE_URL.
func setupPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL não definida — pulando teste de integração")
	}
	pool, err := pgxpool.New(context.Background(), dbURL)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })
	return pool
}

// cleanupJTI remove um JTI da blocklist após o teste (evita poluição entre runs).
func cleanupJTI(t *testing.T, pool *pgxpool.Pool, jti string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		"DELETE FROM token_blocklist WHERE jti = $1", jti)
	require.NoError(t, err)
}

// TestBlocklist_Revoke_IsRevoked: revogar token; IsRevoked retorna true.
func TestBlocklist_Revoke_IsRevoked(t *testing.T) {
	pool := setupPool(t)
	bl := NewDBBlocklist(pool)
	ctx := context.Background()

	jti := "test-jti-revoked-" + t.Name()
	expiresAt := time.Now().Add(15 * time.Minute)
	t.Cleanup(func() { cleanupJTI(t, pool, jti) })

	err := bl.Revoke(ctx, jti, expiresAt, "logout")
	require.NoError(t, err)

	revoked, err := bl.IsRevoked(ctx, jti)
	require.NoError(t, err)
	assert.True(t, revoked, "JTI revogado deve retornar true em IsRevoked")
}

// TestBlocklist_NotRevoked: jti não-revogado; IsRevoked retorna false.
func TestBlocklist_NotRevoked(t *testing.T) {
	pool := setupPool(t)
	bl := NewDBBlocklist(pool)
	ctx := context.Background()

	jti := "test-jti-not-revoked-" + t.Name()

	revoked, err := bl.IsRevoked(ctx, jti)
	require.NoError(t, err)
	assert.False(t, revoked, "JTI não-revogado deve retornar false em IsRevoked")
}

// TestBlocklist_Revoke_Idempotent: revogar o mesmo JTI duas vezes não causa erro.
func TestBlocklist_Revoke_Idempotent(t *testing.T) {
	pool := setupPool(t)
	bl := NewDBBlocklist(pool)
	ctx := context.Background()

	jti := "test-jti-idempotent-" + t.Name()
	expiresAt := time.Now().Add(15 * time.Minute)
	t.Cleanup(func() { cleanupJTI(t, pool, jti) })

	err := bl.Revoke(ctx, jti, expiresAt, "logout")
	require.NoError(t, err)

	// Segunda revogação não deve causar erro (ON CONFLICT DO NOTHING).
	err = bl.Revoke(ctx, jti, expiresAt, "logout-duplicado")
	require.NoError(t, err, "segunda revogação deve ser idempotente")
}

// TestBlocklist_VerifyTokenWithBlocklist_Revoked: emitir token, revogar jti, verificar retorna ErrTokenRevoked.
func TestBlocklist_VerifyTokenWithBlocklist_Revoked(t *testing.T) {
	pool := setupPool(t)
	bl := NewDBBlocklist(pool)
	ctx := context.Background()

	t.Setenv("JWT_SECRET", "secret-de-teste-minimo-32-chars!!")

	// Emitir token real.
	tokenStr, err := IssueAccessToken("user-revoke-test", "gestor", "")
	require.NoError(t, err)

	// Extrair JTI do token emitido.
	claims, err := VerifyToken(tokenStr)
	require.NoError(t, err)
	jti := claims.ID
	require.NotEmpty(t, jti)
	t.Cleanup(func() { cleanupJTI(t, pool, jti) })

	// Verificar que antes da revogação o token é válido.
	verifiedClaims, err := VerifyTokenWithBlocklist(ctx, tokenStr, bl)
	require.NoError(t, err)
	assert.Equal(t, "user-revoke-test", verifiedClaims.Subject)

	// Revogar o JTI.
	err = bl.Revoke(ctx, jti, claims.ExpiresAt.Time, "logout")
	require.NoError(t, err)

	// Verificar que após revogação retorna ErrTokenRevoked.
	_, err = VerifyTokenWithBlocklist(ctx, tokenStr, bl)
	assert.ErrorIs(t, err, ErrTokenRevoked, "token com JTI revogado deve retornar ErrTokenRevoked")
}

// TestBlocklist_VerifyTokenWithBlocklist_Valid: token válido não revogado passa normalmente.
func TestBlocklist_VerifyTokenWithBlocklist_Valid(t *testing.T) {
	pool := setupPool(t)
	bl := NewDBBlocklist(pool)
	ctx := context.Background()

	t.Setenv("JWT_SECRET", "secret-de-teste-minimo-32-chars!!")

	tokenStr, err := IssueAccessToken("user-valid-bl", "financeiro", "")
	require.NoError(t, err)

	claims, err := VerifyTokenWithBlocklist(ctx, tokenStr, bl)
	require.NoError(t, err)
	assert.Equal(t, "user-valid-bl", claims.Subject)
	assert.Equal(t, "financeiro", claims.Role)
}
