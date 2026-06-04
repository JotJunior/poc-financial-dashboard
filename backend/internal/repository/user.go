// Package repository — user repository.
// Lookup de usuário por email para o fluxo de autenticação.
// Constitution P-IV RBAC: o repositório retorna role e vendor_id para o auth layer.
package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrUserNotFound é retornado quando o usuário não existe no banco.
var ErrUserNotFound = errors.New("repository: usuário não encontrado")

// UserRow representa os campos necessários para autenticação.
type UserRow struct {
	ID           string  // UUID
	PasswordHash string
	Role         string
	VendorID     *string // nil quando role != "vendedor"
}

// UserRepository busca dados de usuário para autenticação.
type UserRepository struct {
	pool *pgxpool.Pool
}

// NewUserRepository cria um UserRepository com o pool pgx fornecido.
func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

// FindByEmail retorna o UserRow para o email fornecido.
// Retorna ErrUserNotFound se o email não existir.
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (UserRow, error) {
	const q = `
		SELECT id, password_hash, role, vendor_id
		FROM users
		WHERE email = $1
	`

	var row UserRow
	err := r.pool.QueryRow(ctx, q, email).Scan(
		&row.ID,
		&row.PasswordHash,
		&row.Role,
		&row.VendorID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserRow{}, ErrUserNotFound
		}
		return UserRow{}, fmt.Errorf("repository: buscar usuário por email: %w", err)
	}

	return row, nil
}

// FindByID retorna o UserRow para o UUID fornecido.
// Usado pelo fluxo de refresh token para reconstruir claims com role/vendor_id.
// Retorna ErrUserNotFound se o ID não existir.
func (r *UserRepository) FindByID(ctx context.Context, id string) (UserRow, error) {
	const q = `
		SELECT id, password_hash, role, vendor_id
		FROM users
		WHERE id = $1
	`

	var row UserRow
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&row.ID,
		&row.PasswordHash,
		&row.Role,
		&row.VendorID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserRow{}, ErrUserNotFound
		}
		return UserRow{}, fmt.Errorf("repository: buscar usuário por id: %w", err)
	}

	return row, nil
}
