// Package dto — Commission DTOs com json tags camelCase para a API REST.
// Constitution P-III: valores monetários como int64 (centavos); percentuais como string.
// Constitution P-I: todos os campos de auditoria são expostos (calculatedAt, status).
package dto

import (
	"financial-dashboard/backend/internal/repository"
)

// CommissionResponse é o DTO de saída para uma comissão.
type CommissionResponse struct {
	ID                string `json:"id"`
	OrderID           string `json:"orderId"`
	VendorID          string `json:"vendorId"`
	ValueCents        int64  `json:"valueCents"`        // bruto em centavos (P-III: int64)
	NetCents          int64  `json:"netCents"`          // líquido após estornos (P-III: int64)
	AppliedPercentage string `json:"appliedPercentage"` // "8.0000" — string para preservar precisão
	RuleID            string `json:"ruleId"`
	PeriodYear        int    `json:"periodYear"`
	PeriodMonth       int    `json:"periodMonth"`
	Status            string `json:"status"`
	CalculatedAt      string `json:"calculatedAt"`
}

// CommissionFromDomain converte um repository.Commission para CommissionResponse.
func CommissionFromDomain(c repository.Commission) CommissionResponse {
	return CommissionResponse{
		ID:                c.ID,
		OrderID:           c.OrderID,
		VendorID:          c.VendorID,
		ValueCents:        c.ValueCents,
		NetCents:          c.NetCents,
		AppliedPercentage: c.AppliedPercentage.String(),
		RuleID:            c.RuleID,
		PeriodYear:        c.PeriodYear,
		PeriodMonth:       c.PeriodMonth,
		Status:            string(c.Status),
		CalculatedAt:      c.CalculatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

// ApurationResultResponse é o DTO de saída para o resultado de apuração.
type ApurationResultResponse struct {
	PeriodYear  int   `json:"periodYear"`
	PeriodMonth int   `json:"periodMonth"`
	Calculated  int   `json:"calculated"`   // número de comissões calculadas/inseridas
	Skipped     int   `json:"skipped"`      // número de comissões puladas (idempotência)
	TotalCents  int64 `json:"totalCents"`   // soma das comissões inseridas em centavos (P-III: int64)
}

// TransitionCommissionRequest é o DTO de entrada para transição de status.
type TransitionCommissionRequest struct {
	Status string `json:"status"` // "aprovado", "pago" ou "pendente"
	Motivo string `json:"motivo"` // obrigatório para aprovado→pendente
}

// ApurateRequest é o DTO de entrada para apuração mensal.
type ApurateRequest struct {
	Year  int `json:"year"`
	Month int `json:"month"`
}
