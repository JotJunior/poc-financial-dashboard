// Package dto — Order DTOs com json tags camelCase para a API REST.
// Constitution P-III: totalCents e unitPriceCents como int64 — NUNCA float.
package dto

import (
	"financial-dashboard/backend/internal/repository"
)

// OrderItemResponse é o DTO de saída de um item de pedido.
type OrderItemResponse struct {
	ID             string `json:"id"`
	Description    string `json:"description"`
	Quantity       int    `json:"quantity"`
	UnitPriceCents int64  `json:"unitPriceCents"`
	LineTotalCents int64  `json:"lineTotalCents"`
}

// OrderResponse é o DTO de saída para um pedido.
type OrderResponse struct {
	ID         string              `json:"id"`
	VendorID   string              `json:"vendorId"`
	TotalCents int64               `json:"totalCents"`
	OrderDate  string              `json:"orderDate"`
	Status     string              `json:"status"`
	PaidAt     *string             `json:"paidAt,omitempty"`
	CreatedAt  string              `json:"createdAt"`
	Items      []OrderItemResponse `json:"items"`
}

// OrderFromDomain converte um repository.Order para OrderResponse.
func OrderFromDomain(o repository.Order) OrderResponse {
	resp := OrderResponse{
		ID:         o.ID,
		VendorID:   o.VendorID,
		TotalCents: o.TotalCents,
		OrderDate:  o.OrderDate.Format("2006-01-02"),
		Status:     string(o.Status),
		CreatedAt:  o.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	if o.PaidAt != nil {
		s := o.PaidAt.UTC().Format("2006-01-02T15:04:05Z")
		resp.PaidAt = &s
	}
	for _, item := range o.Items {
		resp.Items = append(resp.Items, OrderItemResponse{
			ID:             item.ID,
			Description:    item.Description,
			Quantity:       item.Quantity,
			UnitPriceCents: item.UnitPriceCents,
			LineTotalCents: item.LineTotalCents,
		})
	}
	return resp
}

// CreateOrderRequest é o DTO de entrada para criação de pedido.
type CreateOrderRequest struct {
	VendorID   string                  `json:"vendorId"`
	TotalCents int64                   `json:"totalCents"` // centavos — nunca float
	OrderDate  string                  `json:"orderDate"`  // "2024-01-15"
	Items      []CreateOrderItemRequest `json:"items"`
}

// CreateOrderItemRequest é o DTO de entrada de um item de pedido.
type CreateOrderItemRequest struct {
	Description    string `json:"description"`
	Quantity       int    `json:"quantity"`
	UnitPriceCents int64  `json:"unitPriceCents"`
	LineTotalCents int64  `json:"lineTotalCents"`
}

// TransitionOrderRequest é o DTO de entrada para transição de status.
type TransitionOrderRequest struct {
	Status string `json:"status"` // "confirmado" | "pago" | "cancelado"
}
