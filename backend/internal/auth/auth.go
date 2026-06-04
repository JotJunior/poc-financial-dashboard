// Package auth implementa autenticação JWT HS256 com Argon2id para hash de senhas.
//
// Constitution P-IV — RBAC deny-by-default.
// OWASP JWT Security: algoritmo fixado em HS256; outros são rejeitados.
// Hash de senha: Argon2id m=64MB (65536 KiB), t=3, p=4 — task 0.5.2 / CHK004.
// Blocklist: campo jti (UUID) presente em todo token para revogação individual (CHK011).
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/argon2"
)

// ─── Erros sentinela ──────────────────────────────────────────────────────────

var (
	// ErrInvalidAlgorithm é retornado quando o token usa algoritmo diferente de HS256.
	ErrInvalidAlgorithm = errors.New("auth: algoritmo JWT inválido — apenas HS256 aceito")

	// ErrTokenExpired é retornado quando o token está expirado.
	ErrTokenExpired = errors.New("auth: token expirado")

	// ErrInvalidToken cobre todos os outros casos de token inválido.
	ErrInvalidToken = errors.New("auth: token inválido")

	// ErrMissingSecret é retornado quando JWT_SECRET não está definido.
	ErrMissingSecret = errors.New("auth: JWT_SECRET não definido no ambiente")
)

// ─── Parâmetros Argon2id ──────────────────────────────────────────────────────

const (
	argonTime    uint32 = 3
	argonMemory  uint32 = 64 * 1024 // 64 MB em KiB
	argonThreads uint8  = 4
	argonKeyLen  uint32 = 32
	saltLen             = 16
)

// ─── Claims JWT ───────────────────────────────────────────────────────────────

// Claims representa o payload JWT do sistema.
// sub = userID; role ∈ {"gestor","vendedor","financeiro"}; vendor_id apenas para papel vendedor.
type Claims struct {
	jwt.RegisteredClaims
	Role     string `json:"role"`
	VendorID string `json:"vendor_id,omitempty"`
}

// ─── Senha ────────────────────────────────────────────────────────────────────

// HashPassword deriva um hash Argon2id da senha em texto plano.
// Formato de saída: "$argon2id$v=19$m=65536,t=3,p=4$<salt-b64>$<hash-b64>"
// Compatível com verificação futura sem recodificação.
func HashPassword(plain string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("auth: gerar salt: %w", err)
	}

	hash := argon2.IDKey([]byte(plain), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	encoded := fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argonMemory,
		argonTime,
		argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)
	return encoded, nil
}

// VerifyPassword verifica se plain corresponde ao hash armazenado (Argon2id).
// Usa comparação de tempo constante para evitar timing attacks.
func VerifyPassword(plain, stored string) bool {
	salt, expected, ok := parseArgon2Hash(stored)
	if !ok {
		return false
	}

	computed := argon2.IDKey([]byte(plain), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return subtle.ConstantTimeCompare(computed, expected) == 1
}

// parseArgon2Hash decodifica o hash no formato "$argon2id$v=19$m=...,t=...,p=...$<salt>$<hash>".
func parseArgon2Hash(encoded string) (salt, hash []byte, ok bool) {
	parts := strings.Split(encoded, "$")
	// partes esperadas: ["", "argon2id", "v=19", "m=65536,t=3,p=4", "<salt>", "<hash>"]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return nil, nil, false
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, nil, false
	}

	hash, err = base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return nil, nil, false
	}

	return salt, hash, true
}

// ─── JWT ─────────────────────────────────────────────────────────────────────

// jwtSecret retorna o segredo JWT do ambiente ou erro se ausente.
func jwtSecret() ([]byte, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, ErrMissingSecret
	}
	return []byte(secret), nil
}

// newJTI gera um JWT ID único (UUID v4 simplificado via crypto/rand).
func newJTI() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("auth: gerar jti: %w", err)
	}
	// Formato UUID v4: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// accessTTL lê JWT_ACCESS_TTL do env; padrão 15 minutos.
func accessTTL() time.Duration {
	if v := os.Getenv("JWT_ACCESS_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return 15 * time.Minute
}

// refreshTTL lê JWT_REFRESH_TTL do env; padrão 168 horas (7 dias).
func refreshTTL() time.Duration {
	if v := os.Getenv("JWT_REFRESH_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return 168 * time.Hour
}

// IssueAccessToken emite um JWT de acesso (TTL=15min por padrão).
// Claims: sub=userID, role, vendor_id (se não vazio), exp, jti.
func IssueAccessToken(userID, role, vendorID string) (string, error) {
	secret, err := jwtSecret()
	if err != nil {
		return "", err
	}

	jti, err := newJTI()
	if err != nil {
		return "", err
	}

	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:  userID,
			IssuedAt: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(accessTTL())),
			ID:       jti,
		},
		Role:     role,
		VendorID: vendorID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(secret)
	if err != nil {
		return "", fmt.Errorf("auth: assinar access token: %w", err)
	}
	return signed, nil
}

// IssueRefreshToken emite um JWT de renovação (TTL=7 dias por padrão).
// Claims: sub=userID, exp, jti. Sem role/vendor_id — apenas para renovar access.
func IssueRefreshToken(userID string) (string, error) {
	secret, err := jwtSecret()
	if err != nil {
		return "", err
	}

	jti, err := newJTI()
	if err != nil {
		return "", err
	}

	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(refreshTTL())),
			ID:        jti,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(secret)
	if err != nil {
		return "", fmt.Errorf("auth: assinar refresh token: %w", err)
	}
	return signed, nil
}

// VerifyToken valida e decodifica um JWT.
// OWASP: rejeita qualquer algoritmo diferente de HS256 (dec-034).
// Retorna ErrInvalidAlgorithm, ErrTokenExpired ou ErrInvalidToken.
func VerifyToken(tokenString string) (*Claims, error) {
	secret, err := jwtSecret()
	if err != nil {
		return nil, err
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		// Pinnar algoritmo — rejeitar qualquer coisa diferente de HS256.
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidAlgorithm
		}
		if t.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidAlgorithm
		}
		return secret, nil
	})

	if err != nil {
		// Distinguir expirado de inválido.
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		if errors.Is(err, ErrInvalidAlgorithm) {
			return nil, ErrInvalidAlgorithm
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	if !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
