// Package http — adaptador de repositório para AuthUserFinder.
// Desacopla o handler http do pacote repository, permitindo injeção
// de mocks nos testes unitários sem dependência de banco de dados.
package http

import (
	"context"
	"fmt"

	"financial-dashboard/backend/internal/repository"
)

// UserAdapter adapta *repository.UserRepository para a interface AuthUserFinder.
type UserAdapter struct {
	repo *repository.UserRepository
}

// NewUserAdapter cria um UserAdapter a partir de um UserRepository.
func NewUserAdapter(repo *repository.UserRepository) *UserAdapter {
	return &UserAdapter{repo: repo}
}

// FindByEmailAuth implementa AuthUserFinder.FindByEmailAuth.
func (a *UserAdapter) FindByEmailAuth(ctx context.Context, email string) (AuthUserRow, error) {
	row, err := a.repo.FindByEmail(ctx, email)
	if err != nil {
		return AuthUserRow{}, fmt.Errorf("user_adapter: %w", err)
	}
	return AuthUserRow{
		ID:           row.ID,
		PasswordHash: row.PasswordHash,
		Role:         row.Role,
		VendorID:     row.VendorID,
	}, nil
}

// FindByIDAuth implementa AuthUserFinder.FindByIDAuth.
func (a *UserAdapter) FindByIDAuth(ctx context.Context, id string) (AuthUserRow, error) {
	row, err := a.repo.FindByID(ctx, id)
	if err != nil {
		return AuthUserRow{}, fmt.Errorf("user_adapter: %w", err)
	}
	return AuthUserRow{
		ID:           row.ID,
		PasswordHash: row.PasswordHash,
		Role:         row.Role,
		VendorID:     row.VendorID,
	}, nil
}
