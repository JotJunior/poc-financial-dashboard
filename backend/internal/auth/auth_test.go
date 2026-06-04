package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Setup ────────────────────────────────────────────────────────────────────

func setJWTSecret(t *testing.T, secret string) {
	t.Helper()
	t.Setenv("JWT_SECRET", secret)
}

// ─── Senha ────────────────────────────────────────────────────────────────────

// TestHashPassword_Verify verifica que hash + verify de senha correta retorna true.
func TestHashPassword_Verify(t *testing.T) {
	plain := "senha-super-segura-2024"
	hash, err := HashPassword(plain)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.True(t, VerifyPassword(plain, hash), "VerifyPassword deve retornar true para senha correta")
}

// TestVerifyPassword_Wrong verifica que senha errada retorna false.
func TestVerifyPassword_Wrong(t *testing.T) {
	hash, err := HashPassword("senha-correta")
	require.NoError(t, err)
	assert.False(t, VerifyPassword("senha-errada", hash), "VerifyPassword deve retornar false para senha errada")
}

// TestHashPassword_DifferentSalts verifica que dois hashes da mesma senha são diferentes (salt aleatório).
func TestHashPassword_DifferentSalts(t *testing.T) {
	plain := "mesma-senha"
	h1, err := HashPassword(plain)
	require.NoError(t, err)
	h2, err := HashPassword(plain)
	require.NoError(t, err)
	assert.NotEqual(t, h1, h2, "cada hash deve usar salt único")
}

// TestVerifyPassword_InvalidHash verifica que hash malformado retorna false sem panic.
func TestVerifyPassword_InvalidHash(t *testing.T) {
	assert.False(t, VerifyPassword("qualquer", "hash-malformado"))
	assert.False(t, VerifyPassword("qualquer", ""))
	assert.False(t, VerifyPassword("qualquer", "$bcrypt$valor"))
}

// ─── JWT — IssueAccessToken ───────────────────────────────────────────────────

// TestIssueAccessToken_Valid verifica que token emitido contém sub/role/vendor_id e jti.
func TestIssueAccessToken_Valid(t *testing.T) {
	setJWTSecret(t, "secret-de-teste-minimo-32-chars!!")

	tokenStr, err := IssueAccessToken("user-123", "gestor", "vendor-456")
	require.NoError(t, err)
	assert.NotEmpty(t, tokenStr)

	claims, err := VerifyToken(tokenStr)
	require.NoError(t, err)

	assert.Equal(t, "user-123", claims.Subject, "sub deve ser userID")
	assert.Equal(t, "gestor", claims.Role, "role deve estar no claims")
	assert.Equal(t, "vendor-456", claims.VendorID, "vendor_id deve estar no claims")
	assert.NotEmpty(t, claims.ID, "jti deve ser não-vazio")
}

// TestIssueAccessToken_NoVendorID verifica emissão para papel sem vendor_id.
func TestIssueAccessToken_NoVendorID(t *testing.T) {
	setJWTSecret(t, "secret-de-teste-minimo-32-chars!!")

	tokenStr, err := IssueAccessToken("user-789", "financeiro", "")
	require.NoError(t, err)

	claims, err := VerifyToken(tokenStr)
	require.NoError(t, err)

	assert.Equal(t, "financeiro", claims.Role)
	assert.Empty(t, claims.VendorID)
	assert.NotEmpty(t, claims.ID, "jti sempre presente")
}

// TestIssueRefreshToken_Valid verifica refresh token com sub e jti.
func TestIssueRefreshToken_Valid(t *testing.T) {
	setJWTSecret(t, "secret-de-teste-minimo-32-chars!!")

	tokenStr, err := IssueRefreshToken("user-123")
	require.NoError(t, err)
	assert.NotEmpty(t, tokenStr)

	claims, err := VerifyToken(tokenStr)
	require.NoError(t, err)

	assert.Equal(t, "user-123", claims.Subject)
	assert.NotEmpty(t, claims.ID, "jti presente no refresh token")
}

// ─── JWT — VerifyToken ────────────────────────────────────────────────────────

// TestVerifyToken_Expired verifica que token expirado retorna ErrTokenExpired.
func TestVerifyToken_Expired(t *testing.T) {
	secret := "secret-de-teste-minimo-32-chars!!"
	setJWTSecret(t, secret)

	// Emitir token com exp no passado.
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-exp",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			ID:        "jti-test",
		},
		Role: "gestor",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(secret))
	require.NoError(t, err)

	_, err = VerifyToken(tokenStr)
	assert.ErrorIs(t, err, ErrTokenExpired, "token expirado deve retornar ErrTokenExpired")
}

// TestVerifyToken_WrongAlg verifica que token com alg=RS256 retorna ErrInvalidAlgorithm.
func TestVerifyToken_WrongAlg(t *testing.T) {
	setJWTSecret(t, "secret-de-teste-minimo-32-chars!!")

	// Gerar chave RSA para assinar com RS256.
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-rs256",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			ID:        "jti-rs256",
		},
		Role: "gestor",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenStr, err := token.SignedString(rsaKey)
	require.NoError(t, err)

	_, err = VerifyToken(tokenStr)
	assert.ErrorIs(t, err, ErrInvalidAlgorithm, "alg=RS256 deve retornar ErrInvalidAlgorithm")
}

// TestVerifyToken_ValidClaims verifica que token fresco verifica e retorna claims corretos.
func TestVerifyToken_ValidClaims(t *testing.T) {
	setJWTSecret(t, "secret-de-teste-minimo-32-chars!!")

	tokenStr, err := IssueAccessToken("user-fresh", "vendedor", "vendor-999")
	require.NoError(t, err)

	claims, err := VerifyToken(tokenStr)
	require.NoError(t, err)

	assert.Equal(t, "user-fresh", claims.Subject)
	assert.Equal(t, "vendedor", claims.Role)
	assert.Equal(t, "vendor-999", claims.VendorID)
	assert.NotEmpty(t, claims.ID)
	assert.True(t, claims.ExpiresAt.After(time.Now()), "token fresco não deve estar expirado")
}

// TestVerifyToken_Tampered verifica que token com assinatura adulterada é rejeitado.
func TestVerifyToken_Tampered(t *testing.T) {
	setJWTSecret(t, "secret-de-teste-minimo-32-chars!!")

	tokenStr, err := IssueAccessToken("user-123", "gestor", "")
	require.NoError(t, err)

	// Adulterar o token (inverter últimos 4 chars da assinatura).
	runes := []rune(tokenStr)
	l := len(runes)
	runes[l-1], runes[l-2] = runes[l-2], runes[l-1]
	tampered := string(runes)

	_, err = VerifyToken(tampered)
	assert.Error(t, err, "token adulterado deve ser rejeitado")
}

// TestVerifyToken_MissingSecret verifica erro quando JWT_SECRET está ausente.
func TestVerifyToken_MissingSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "")

	_, err := IssueAccessToken("user", "gestor", "")
	assert.ErrorIs(t, err, ErrMissingSecret)
}
