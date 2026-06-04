// Package dto — Vendor DTOs com json tags camelCase para a API REST.
// Constitution P-III: valores monetários como int64 (centavos); percentuais como string.
package dto

import (
	"financial-dashboard/backend/internal/repository"
)

// VendorResponse é o DTO de saída para um vendedor.
type VendorResponse struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Email        string  `json:"email"`
	Status       string  `json:"status"`
	AnonymizedAt *string `json:"anonymizedAt,omitempty"`
	CreatedAt    string  `json:"createdAt"`
}

// VendorFromDomain converte um repository.Vendor para VendorResponse.
func VendorFromDomain(v repository.Vendor) VendorResponse {
	resp := VendorResponse{
		ID:        v.ID,
		Name:      v.Name,
		Email:     v.Email,
		Status:    string(v.Status),
		CreatedAt: v.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	if v.AnonymizedAt != nil {
		s := v.AnonymizedAt.UTC().Format("2006-01-02T15:04:05Z")
		resp.AnonymizedAt = &s
	}
	return resp
}

// CreateVendorRequest é o DTO de entrada para criação de vendedor.
type CreateVendorRequest struct {
	Name                 string `json:"name"`
	Email                string `json:"email"`
	CommissionPercentage string `json:"commissionPercentage"` // "8.5000" — string para preservar precisão
}

// UpdateVendorRequest é o DTO de entrada para atualização parcial de vendedor.
type UpdateVendorRequest struct {
	Name       *string `json:"name,omitempty"`
	Email      *string `json:"email,omitempty"`
	Status     *string `json:"status,omitempty"`
	Percentage *string `json:"commissionPercentage,omitempty"`
	Confirm    *bool   `json:"confirm,omitempty"` // para DELETE /vendors/{id} (anonimização LGPD)
}
