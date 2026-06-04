// Package service — dashboard service.
// Task 6.2.1: DashboardService aplica RBAC (P-IV):
//   - Gestor/Financeiro: visão consolidada de todos os vendedores.
//   - Vendedor: apenas os próprios dados (vendor_id do JWT é a barreira).
// Constitution P-III: valores em centavos — sem float.
// Constitution P-IV: RBAC deny-by-default; servidor é a barreira real.
package service

import (
	"context"
	"errors"
	"fmt"

	"financial-dashboard/backend/internal/repository"
)

// DashboardService expõe métricas respeitando RBAC.
type DashboardService struct {
	repo repository.DashboardRepository
}

// NewDashboardService cria um DashboardService.
func NewDashboardService(repo repository.DashboardRepository) *DashboardService {
	return &DashboardService{repo: repo}
}

// GetConsolidated retorna dashboard consolidado — apenas Gestor e Financeiro.
// Task 6.2.2.
func (s *DashboardService) GetConsolidated(ctx context.Context, filter repository.DashboardFilter, actorRole string) (repository.ConsolidatedDashboard, error) {
	switch actorRole {
	case "gestor", "financeiro":
		// autorizado
	default:
		return repository.ConsolidatedDashboard{}, fmt.Errorf("%w: papel '%s' não pode acessar dashboard consolidado", ErrForbidden, actorRole)
	}
	return s.repo.GetConsolidated(ctx, filter)
}

// GetVendorDashboard retorna dashboard de um vendedor específico.
// Task 6.2.3 — Vendedor vê apenas os próprios; Gestor pode filtrar por vendorId.
// P-IV: o actorVendorID (do JWT) sobrescreve o requestedVendorID quando o papel é "vendedor".
func (s *DashboardService) GetVendorDashboard(ctx context.Context, requestedVendorID string, filter repository.DashboardFilter, actorRole, actorVendorID string) (repository.VendorDashboard, error) {
	switch actorRole {
	case "vendedor":
		// Vendedor só pode ver os próprios dados — ignora requestedVendorID
		if actorVendorID == "" {
			return repository.VendorDashboard{}, fmt.Errorf("%w: vendedor sem vendor_id no token", ErrForbidden)
		}
		return s.repo.GetVendorDashboard(ctx, actorVendorID, filter)

	case "gestor", "financeiro":
		if requestedVendorID == "" {
			return repository.VendorDashboard{}, errors.New("service: parâmetro vendorId obrigatório para gestor/financeiro")
		}
		return s.repo.GetVendorDashboard(ctx, requestedVendorID, filter)

	default:
		return repository.VendorDashboard{}, fmt.Errorf("%w: papel '%s' não autorizado", ErrForbidden, actorRole)
	}
}

// GetPendingCommissions retorna indicadores de comissões pendentes (FR-019).
// Task 6.2.4 — apenas Gestor e Financeiro.
func (s *DashboardService) GetPendingCommissions(ctx context.Context, actorRole string) (repository.PendingCommissionsSummary, error) {
	switch actorRole {
	case "gestor", "financeiro":
	default:
		return repository.PendingCommissionsSummary{}, fmt.Errorf("%w: papel '%s' não pode acessar comissões pendentes", ErrForbidden, actorRole)
	}
	return s.repo.GetPendingCommissionsSummary(ctx)
}

// GetDrillDown retorna rastreabilidade pedido → comissões → estornos (SC-004).
// Task 6.2.5 — qualquer papel autenticado pode acessar, com escopo de vendedor.
// Vendedor só pode ver pedidos próprios (verificação feita no handler via order.vendor_id).
func (s *DashboardService) GetDrillDown(ctx context.Context, orderID, actorRole, actorVendorID string) (repository.DrillDown, error) {
	dd, err := s.repo.GetDrillDown(ctx, orderID)
	if err != nil {
		return repository.DrillDown{}, err
	}

	// P-IV escopo de vendedor: vendedor só pode ver os próprios pedidos
	if actorRole == "vendedor" && dd.VendorID != actorVendorID {
		return repository.DrillDown{}, fmt.Errorf("%w: vendedor sem acesso a este pedido", ErrForbidden)
	}

	return dd, nil
}
