// Package service — VendorService: lógica de negócio de vendedores.
// Constitution P-IV RBAC deny-by-default: verificação de papel em TODAS as operações.
// Constitution P-V LGPD: anonimização apenas por Gestor.
// FR-003: atualização de percentual cria nova versão de regra de comissão.
package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/shopspring/decimal"

	"financial-dashboard/backend/internal/repository"
)

// ─── Erros ────────────────────────────────────────────────────────────────────

var (
	// ErrForbidden é retornado quando o papel do ator não autoriza a operação.
	ErrForbidden = errors.New("service: operação não autorizada para este papel")

	// ErrInvalidPercentage é retornado para percentual fora do range [0, 100].
	ErrInvalidPercentage = errors.New("service: percentual deve estar entre 0 e 100")

	// ErrInvalidInput é retornado para campos obrigatórios ausentes ou inválidos.
	ErrInvalidInput = errors.New("service: input inválido")
)

// ─── Requests/Responses ───────────────────────────────────────────────────────

// CreateVendorReq agrupa os campos para criação de um vendedor.
type CreateVendorReq struct {
	Name                string
	Email               string
	CommissionPercentage decimal.Decimal // percentual inicial [0, 100]
}

// UpdateVendorReq agrupa os campos para atualização parcial de um vendedor.
type UpdateVendorReq struct {
	Name       *string
	Email      *string
	Status     *repository.VendorStatus
	Percentage *decimal.Decimal // se não-nil, cria nova versão de regra
}

// ─── Interfaces de repositório ────────────────────────────────────────────────

// VendorRepoI é a interface do repositório de vendedores usada pelo service.
type VendorRepoI interface {
	Create(ctx context.Context, vendor repository.Vendor) (repository.Vendor, error)
	FindByID(ctx context.Context, id string) (repository.Vendor, error)
	List(ctx context.Context, filter repository.VendorFilter) ([]repository.Vendor, error)
	Update(ctx context.Context, id string, patch repository.VendorPatch) (repository.Vendor, error)
	Anonymize(ctx context.Context, id string, actorID string) error
}

// CommissionRuleRepoI é a interface do repositório de regras de comissão.
type CommissionRuleRepoI interface {
	CreateRule(ctx context.Context, vendorID string, percentage decimal.Decimal) (repository.CommissionRule, error)
	FindActiveAtDate(ctx context.Context, vendorID string, date interface{}) (repository.CommissionRule, error)
	ListByVendor(ctx context.Context, vendorID string) ([]repository.CommissionRule, error)
}

// BlocklistRevoker revoga todos os tokens de um usuário (para desativação).
type BlocklistRevoker interface {
	RevokeAllForVendor(ctx context.Context, vendorID string, reason string) error
}

// ─── VendorService ────────────────────────────────────────────────────────────

// VendorService implementa a lógica de negócio de vendedores.
type VendorService struct {
	vendors   VendorRepoI
	rules     *repository.PGCommissionRuleRepository
	blocklist BlocklistRevoker // pode ser nil se revogação não estiver disponível
}

// NewVendorService cria um VendorService com as dependências fornecidas.
// blocklist pode ser nil — desativação funcionará mas não revogará tokens automaticamente.
func NewVendorService(
	vendors VendorRepoI,
	rules *repository.PGCommissionRuleRepository,
	blocklist BlocklistRevoker,
) *VendorService {
	return &VendorService{
		vendors:   vendors,
		rules:     rules,
		blocklist: blocklist,
	}
}

// ─── CreateVendor ─────────────────────────────────────────────────────────────

// CreateVendor cria um vendedor com a regra de comissão inicial.
// RBAC: apenas Gestor pode criar vendedores (constitution P-IV).
// A criação do vendedor e da regra de comissão são operações separadas;
// a atomicidade entre elas é garantida pela verificação de email único no banco.
func (s *VendorService) CreateVendor(ctx context.Context, req CreateVendorReq, actorRole string) (repository.Vendor, error) {
	// RBAC: apenas Gestor.
	if actorRole != "gestor" {
		return repository.Vendor{}, ErrForbidden
	}

	// Validar campos obrigatórios.
	if req.Name == "" || req.Email == "" {
		return repository.Vendor{}, fmt.Errorf("%w: name e email são obrigatórios", ErrInvalidInput)
	}
	if len(req.Name) > 200 {
		return repository.Vendor{}, fmt.Errorf("%w: name deve ter no máximo 200 caracteres", ErrInvalidInput)
	}

	// Validar percentual.
	if err := validatePercentage(req.CommissionPercentage); err != nil {
		return repository.Vendor{}, err
	}

	// Criar vendedor.
	vendor, err := s.vendors.Create(ctx, repository.Vendor{
		Name:   req.Name,
		Email:  req.Email,
		Status: repository.VendorStatusAtivo,
	})
	if err != nil {
		return repository.Vendor{}, fmt.Errorf("service: criar vendedor: %w", err)
	}

	// Criar regra de comissão inicial.
	_, err = s.rules.CreateRule(ctx, vendor.ID, req.CommissionPercentage)
	if err != nil {
		// Vendedor foi criado mas regra falhou — log e retornar o vendedor criado.
		// O operador pode criar a regra depois via UpdateVendor.
		return vendor, fmt.Errorf("service: criar regra inicial (vendedor criado): %w", err)
	}

	return vendor, nil
}

// ─── GetVendor ────────────────────────────────────────────────────────────────

// GetVendor retorna o vendedor pelo ID.
// RBAC: Gestor e Financeiro veem qualquer vendedor; Vendedor vê apenas a si mesmo.
func (s *VendorService) GetVendor(ctx context.Context, id string, actorRole string, actorVendorID string) (repository.Vendor, error) {
	if actorRole == "vendedor" && actorVendorID != id {
		return repository.Vendor{}, ErrForbidden
	}
	return s.vendors.FindByID(ctx, id)
}

// ─── ListVendors ──────────────────────────────────────────────────────────────

// ListVendors lista vendedores com filtro opcional por status.
// RBAC: apenas Gestor e Financeiro.
func (s *VendorService) ListVendors(ctx context.Context, filter repository.VendorFilter, actorRole string) ([]repository.Vendor, error) {
	if actorRole != "gestor" && actorRole != "financeiro" {
		return nil, ErrForbidden
	}
	return s.vendors.List(ctx, filter)
}

// ─── UpdateVendor ─────────────────────────────────────────────────────────────

// UpdateVendor aplica patch ao vendedor.
// RBAC: apenas Gestor.
// Se req.Percentage não-nil, cria nova versão de regra de comissão (FR-003).
// Registra em audit_trail: a criação de nova regra é registro no commission_rules
// (histórico imutável conforme P-I).
func (s *VendorService) UpdateVendor(ctx context.Context, id string, req UpdateVendorReq, actorRole string) (repository.Vendor, error) {
	if actorRole != "gestor" {
		return repository.Vendor{}, ErrForbidden
	}

	if req.Name != nil && len(*req.Name) > 200 {
		return repository.Vendor{}, fmt.Errorf("%w: name deve ter no máximo 200 caracteres", ErrInvalidInput)
	}

	if req.Percentage != nil {
		if err := validatePercentage(*req.Percentage); err != nil {
			return repository.Vendor{}, err
		}
	}

	patch := repository.VendorPatch{
		Name:   req.Name,
		Email:  req.Email,
		Status: req.Status,
	}

	vendor, err := s.vendors.Update(ctx, id, patch)
	if err != nil {
		return repository.Vendor{}, fmt.Errorf("service: atualizar vendedor: %w", err)
	}

	// Criar nova versão de regra se percentual foi alterado (FR-003).
	if req.Percentage != nil {
		_, err = s.rules.CreateRule(ctx, id, *req.Percentage)
		if err != nil {
			return vendor, fmt.Errorf("service: criar nova regra de comissão: %w", err)
		}
	}

	return vendor, nil
}

// ─── DeactivateVendor ────────────────────────────────────────────────────────

// DeactivateVendor marca o vendedor como inativo e revoga seus tokens JWT.
// RBAC: apenas Gestor.
// CHK011: tokens são revogados via blocklist para evitar uso após desativação.
func (s *VendorService) DeactivateVendor(ctx context.Context, id string, actorRole string) error {
	if actorRole != "gestor" {
		return ErrForbidden
	}

	status := repository.VendorStatusInativo
	_, err := s.vendors.Update(ctx, id, repository.VendorPatch{Status: &status})
	if err != nil {
		return fmt.Errorf("service: desativar vendedor: %w", err)
	}

	// Revogar tokens JWT do vendedor (CHK011).
	if s.blocklist != nil {
		if err := s.blocklist.RevokeAllForVendor(ctx, id, "vendor_deactivated"); err != nil {
			// Não falhar a desativação — logar e seguir.
			// O token expirará naturalmente em até 15 minutos.
			return fmt.Errorf("service: desativar vendedor (tokens não revogados): %w", err)
		}
	}

	return nil
}

// ─── AnonymizeVendor ─────────────────────────────────────────────────────────

// AnonymizeVendor anonimiza PII do vendedor (LGPD).
// RBAC: apenas Gestor.
// CHK077: body deve conter {"confirm":true} — validado no handler.
func (s *VendorService) AnonymizeVendor(ctx context.Context, id string, actorUserID string, actorRole string) error {
	if actorRole != "gestor" {
		return ErrForbidden
	}
	return s.vendors.Anonymize(ctx, id, actorUserID)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func validatePercentage(p decimal.Decimal) error {
	if p.IsNegative() || p.GreaterThan(decimal.NewFromInt(100)) {
		return fmt.Errorf("%w: %s", ErrInvalidPercentage, p.String())
	}
	return nil
}
