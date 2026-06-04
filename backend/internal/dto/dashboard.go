// Package dto — Dashboard DTOs com json tags camelCase para a API REST.
// Task 6.2: DTOs de saída para dashboard/metricas endpoints.
// Constitution P-III: valores monetários como int64 (centavos) — zero float.
package dto

import (
	"financial-dashboard/backend/internal/repository"
)

// ─── Consolidated Dashboard ───────────────────────────────────────────────────

// VendorRankingResponse é o item do ranking de vendedores.
type VendorRankingResponse struct {
	VendorID    string `json:"vendorId"`
	VendorName  string `json:"vendorName"`
	TotalCents  int64  `json:"totalCents"`  // pedidos pagos em centavos (P-III)
	OrderCount  int    `json:"orderCount"`
	CommCents   int64  `json:"commCents"`   // comissões net em centavos (P-III)
}

// ConsolidatedDashboardResponse é o DTO de saída do endpoint GET /dashboard/consolidated.
type ConsolidatedDashboardResponse struct {
	TotalSalesCents   int64                   `json:"totalSalesCents"`   // P-III
	TotalCommCents    int64                   `json:"totalCommCents"`    // P-III
	PendingCommCents  int64                   `json:"pendingCommCents"`  // P-III
	ApprovedCommCents int64                   `json:"approvedCommCents"` // P-III
	PaidCommCents     int64                   `json:"paidCommCents"`     // P-III
	OrderCount        int                     `json:"orderCount"`
	VendorCount       int                     `json:"vendorCount"`
	TopVendors        []VendorRankingResponse  `json:"topVendors"`
}

// ConsolidatedDashboardFromDomain converte repository.ConsolidatedDashboard para DTO.
func ConsolidatedDashboardFromDomain(d repository.ConsolidatedDashboard) ConsolidatedDashboardResponse {
	topVendors := make([]VendorRankingResponse, 0, len(d.TopVendors))
	for _, vr := range d.TopVendors {
		topVendors = append(topVendors, VendorRankingResponse{
			VendorID:   vr.VendorID,
			VendorName: vr.VendorName,
			TotalCents: vr.TotalCents,
			OrderCount: vr.OrderCount,
			CommCents:  vr.CommCents,
		})
	}
	return ConsolidatedDashboardResponse{
		TotalSalesCents:   d.TotalSalesCents,
		TotalCommCents:    d.TotalCommCents,
		PendingCommCents:  d.PendingCommCents,
		ApprovedCommCents: d.ApprovedCommCents,
		PaidCommCents:     d.PaidCommCents,
		OrderCount:        d.OrderCount,
		VendorCount:       d.VendorCount,
		TopVendors:        topVendors,
	}
}

// ─── Vendor Dashboard ─────────────────────────────────────────────────────────

// VendorDashboardResponse é o DTO de saída do endpoint GET /dashboard/vendor.
type VendorDashboardResponse struct {
	VendorID          string `json:"vendorId"`
	TotalSalesCents   int64  `json:"totalSalesCents"`   // P-III
	TotalCommCents    int64  `json:"totalCommCents"`    // P-III
	PendingCommCents  int64  `json:"pendingCommCents"`  // P-III
	ApprovedCommCents int64  `json:"approvedCommCents"` // P-III
	PaidCommCents     int64  `json:"paidCommCents"`     // P-III
	OrderCount        int    `json:"orderCount"`
}

// VendorDashboardFromDomain converte repository.VendorDashboard para DTO.
func VendorDashboardFromDomain(d repository.VendorDashboard) VendorDashboardResponse {
	return VendorDashboardResponse{
		VendorID:          d.VendorID,
		TotalSalesCents:   d.TotalSalesCents,
		TotalCommCents:    d.TotalCommCents,
		PendingCommCents:  d.PendingCommCents,
		ApprovedCommCents: d.ApprovedCommCents,
		PaidCommCents:     d.PaidCommCents,
		OrderCount:        d.OrderCount,
	}
}

// ─── Pending Commissions ──────────────────────────────────────────────────────

// PendingCommissionsResponse é o DTO de saída do endpoint GET /dashboard/commissions/pending.
type PendingCommissionsResponse struct {
	PendingApprovalCount int   `json:"pendingApprovalCount"`
	PendingApprovalCents int64 `json:"pendingApprovalCents"` // P-III
	ApprovedUnpaidCount  int   `json:"approvedUnpaidCount"`
	ApprovedUnpaidCents  int64 `json:"approvedUnpaidCents"`  // P-III
}

// PendingCommissionsFromDomain converte repository.PendingCommissionsSummary para DTO.
func PendingCommissionsFromDomain(s repository.PendingCommissionsSummary) PendingCommissionsResponse {
	return PendingCommissionsResponse{
		PendingApprovalCount: s.PendingApprovalCount,
		PendingApprovalCents: s.PendingApprovalCents,
		ApprovedUnpaidCount:  s.ApprovedUnpaidCount,
		ApprovedUnpaidCents:  s.ApprovedUnpaidCents,
	}
}

// ─── Drill Down ───────────────────────────────────────────────────────────────

// DrillDownReversalResponse é o DTO de estorno dentro de um drilldown.
type DrillDownReversalResponse struct {
	ReversalID  string `json:"reversalId"`
	ValueCents  int64  `json:"valueCents"`  // negativo (P-III)
	Status      string `json:"status"`
	CreatedAt   string `json:"createdAt"`
	ActorUserID string `json:"actorUserId"` // auditabilidade P-I
}

// DrillDownCommissionResponse é o DTO de comissão dentro de um drilldown.
type DrillDownCommissionResponse struct {
	CommissionID string                      `json:"commissionId"`
	ValueCents   int64                       `json:"valueCents"` // P-III
	NetCents     int64                       `json:"netCents"`   // P-III
	Status       string                      `json:"status"`
	PeriodYear   int                         `json:"periodYear"`
	PeriodMonth  int                         `json:"periodMonth"`
	Reversals    []DrillDownReversalResponse `json:"reversals"`
}

// DrillDownResponse é o DTO de saída do endpoint GET /orders/{id}/drilldown.
type DrillDownResponse struct {
	OrderID     string                        `json:"orderId"`
	VendorID    string                        `json:"vendorId"`
	TotalCents  int64                         `json:"totalCents"` // P-III
	OrderDate   string                        `json:"orderDate"`  // "YYYY-MM-DD"
	Status      string                        `json:"status"`
	CreatedAt   string                        `json:"createdAt"`
	Commissions []DrillDownCommissionResponse `json:"commissions"`
}

// DrillDownFromDomain converte repository.DrillDown para DTO.
func DrillDownFromDomain(dd repository.DrillDown) DrillDownResponse {
	comms := make([]DrillDownCommissionResponse, 0, len(dd.Commissions))
	for _, c := range dd.Commissions {
		revs := make([]DrillDownReversalResponse, 0, len(c.Reversals))
		for _, rv := range c.Reversals {
			revs = append(revs, DrillDownReversalResponse{
				ReversalID:  rv.ReversalID,
				ValueCents:  rv.ValueCents,
				Status:      rv.Status,
				CreatedAt:   rv.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
				ActorUserID: rv.ActorUserID,
			})
		}
		comms = append(comms, DrillDownCommissionResponse{
			CommissionID: c.CommissionID,
			ValueCents:   c.ValueCents,
			NetCents:     c.NetCents,
			Status:       c.Status,
			PeriodYear:   c.PeriodYear,
			PeriodMonth:  c.PeriodMonth,
			Reversals:    revs,
		})
	}
	return DrillDownResponse{
		OrderID:     dd.OrderID,
		VendorID:    dd.VendorID,
		TotalCents:  dd.TotalCents,
		OrderDate:   dd.OrderDate.Format("2006-01-02"),
		Status:      dd.Status,
		CreatedAt:   dd.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		Commissions: comms,
	}
}
