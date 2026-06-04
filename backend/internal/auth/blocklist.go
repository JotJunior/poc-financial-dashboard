package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TokenBlocklist define a interface para revogar e verificar JTIs de tokens JWT.
// Implementação via PostgreSQL (DBBlocklist); pode ser substituída por Redis futuramente.
// CHK011: necessário porque JWT é stateless e não tem revogação nativa.
type TokenBlocklist interface {
	// Revoke insere o JTI na blocklist. Idempotente: ON CONFLICT DO NOTHING.
	Revoke(ctx context.Context, jti string, expiresAt time.Time, reason string) error

	// IsRevoked retorna true se o JTI foi revogado, false caso contrário.
	IsRevoked(ctx context.Context, jti string) (bool, error)
}

// DBBlocklist implementa TokenBlocklist usando PostgreSQL (tabela token_blocklist).
type DBBlocklist struct {
	pool *pgxpool.Pool
}

// NewDBBlocklist cria um DBBlocklist conectado ao pool pgx fornecido.
func NewDBBlocklist(pool *pgxpool.Pool) *DBBlocklist {
	return &DBBlocklist{pool: pool}
}

// Revoke insere o JTI na blocklist com sua data de expiração e motivo.
// Usa ON CONFLICT DO NOTHING para idempotência (revogar duas vezes não é erro).
func (b *DBBlocklist) Revoke(ctx context.Context, jti string, expiresAt time.Time, reason string) error {
	const q = `
		INSERT INTO token_blocklist (jti, expires_at, reason)
		VALUES ($1, $2, $3)
		ON CONFLICT (jti) DO NOTHING
	`
	_, err := b.pool.Exec(ctx, q, jti, expiresAt, reason)
	if err != nil {
		return fmt.Errorf("blocklist: revogar jti %q: %w", jti, err)
	}
	return nil
}

// IsRevoked verifica se o JTI está na blocklist.
// Retorna true se revogado, false caso contrário.
func (b *DBBlocklist) IsRevoked(ctx context.Context, jti string) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM token_blocklist WHERE jti = $1)`
	var revoked bool
	if err := b.pool.QueryRow(ctx, q, jti).Scan(&revoked); err != nil {
		return false, fmt.Errorf("blocklist: verificar jti %q: %w", jti, err)
	}
	return revoked, nil
}

// VerifyTokenWithBlocklist valida o JWT e verifica se o JTI foi revogado.
// Combina VerifyToken (assinatura + exp + alg) com checagem de blocklist.
// Retorna ErrTokenRevoked se o token foi explicitamente revogado.
func VerifyTokenWithBlocklist(ctx context.Context, tokenString string, bl TokenBlocklist) (*Claims, error) {
	claims, err := VerifyToken(tokenString)
	if err != nil {
		return nil, err
	}

	// Verificar se o JTI foi revogado.
	if claims.ID != "" {
		revoked, err := bl.IsRevoked(ctx, claims.ID)
		if err != nil {
			return nil, fmt.Errorf("auth: verificar blocklist: %w", err)
		}
		if revoked {
			return nil, ErrTokenRevoked
		}
	}

	return claims, nil
}
