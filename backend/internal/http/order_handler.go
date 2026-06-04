// Package http — handlers HTTP de pedidos.
// Task 4.3: GET/POST /api/v1/orders, GET /api/v1/orders/{id}, PATCH /api/v1/orders/{id}/status
// RBAC: RequireAuth + RequireRole conforme constitution P-IV.
// Constitution P-III: totalCents é int64 — nunca float em DTOs.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"financial-dashboard/backend/internal/domain"
	"financial-dashboard/backend/internal/dto"
	"financial-dashboard/backend/internal/http/middleware"
	"financial-dashboard/backend/internal/repository"
	"financial-dashboard/backend/internal/service"
)

// ─── Interface do service ──────────────────────────────────────────────────────

// OrderServiceI abstrai OrderService para facilitar testes.
type OrderServiceI interface {
	CreateOrder(ctx context.Context, req service.CreateOrderReq, actorRole string) (repository.Order, error)
	GetOrder(ctx context.Context, id string) (repository.Order, error)
	ListOrders(ctx context.Context, filter repository.OrderFilter) ([]repository.Order, error)
	TransitionOrder(ctx context.Context, req service.TransitionOrderReq) (repository.Order, error)
}

// ─── OrderHandler ────────────────────────────────────────────────────────────

// OrderHandler agrupa os handlers de /orders/*.
type OrderHandler struct {
	svc OrderServiceI
}

// NewOrderHandler cria um OrderHandler.
func NewOrderHandler(svc OrderServiceI) *OrderHandler {
	return &OrderHandler{svc: svc}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func writeOrderJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

type orderErrJSON struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

func orderClaimsFromReq(r *http.Request) (role, vendorID, userID string) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		return "", "", ""
	}
	return claims.Role, claims.VendorID, claims.Subject
}

// ─── List — GET /api/v1/orders ────────────────────────────────────────────────

// List retorna pedidos com filtros opcionais.
// Gestor e Financeiro vêem todos; Vendedor vê apenas seus pedidos (escopo por vendor_id).
// Query params: ?vendorId=, ?status=, ?year=, ?month=
func (h *OrderHandler) List(w http.ResponseWriter, r *http.Request) {
	role, vendorID, _ := orderClaimsFromReq(r)

	filter := repository.OrderFilter{}

	// Vendedor só pode ver seus próprios pedidos (P-IV RBAC).
	if role == "vendedor" {
		if vendorID == "" {
			writeOrderJSON(w, http.StatusForbidden, orderErrJSON{Error: "forbidden", Message: "vendedor sem vendor_id"})
			return
		}
		filter.VendorID = &vendorID
	} else if v := r.URL.Query().Get("vendorId"); v != "" {
		filter.VendorID = &v
	}

	if s := r.URL.Query().Get("status"); s != "" {
		st := domain.OrderStatus(s)
		filter.Status = &st
	}

	orders, err := h.svc.ListOrders(r.Context(), filter)
	if err != nil {
		slog.Error("order: listar", "err", err)
		writeOrderJSON(w, http.StatusInternalServerError, orderErrJSON{Error: "internal_error"})
		return
	}

	resp := make([]dto.OrderResponse, len(orders))
	for i, o := range orders {
		resp[i] = dto.OrderFromDomain(o)
	}
	writeOrderJSON(w, http.StatusOK, resp)
}

// ─── Create — POST /api/v1/orders ─────────────────────────────────────────────

// Create cria um pedido com status 'rascunho'.
// Apenas Gestor. totalCents deve ser int64 >= 0 (P-III).
func (h *OrderHandler) Create(w http.ResponseWriter, r *http.Request) {
	role, _, _ := orderClaimsFromReq(r)

	var req dto.CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeOrderJSON(w, http.StatusBadRequest, orderErrJSON{Error: "invalid_json", Message: err.Error()})
		return
	}

	// Validar e parsear order_date.
	if req.OrderDate == "" {
		writeOrderJSON(w, http.StatusBadRequest, orderErrJSON{Error: "validation_error", Message: "orderDate obrigatório (YYYY-MM-DD)"})
		return
	}
	orderDate, err := time.Parse("2006-01-02", req.OrderDate)
	if err != nil {
		writeOrderJSON(w, http.StatusBadRequest, orderErrJSON{Error: "validation_error", Message: "orderDate deve estar no formato YYYY-MM-DD"})
		return
	}

	// Mapear items do DTO para repository.
	items := make([]repository.CreateOrderItemReq, len(req.Items))
	for i, it := range req.Items {
		items[i] = repository.CreateOrderItemReq{
			Description:    it.Description,
			Quantity:       it.Quantity,
			UnitPriceCents: it.UnitPriceCents,
			LineTotalCents: it.LineTotalCents,
		}
	}

	order, err := h.svc.CreateOrder(r.Context(), service.CreateOrderReq{
		VendorID:   req.VendorID,
		TotalCents: req.TotalCents,
		OrderDate:  orderDate,
		Items:      items,
	}, role)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrForbidden):
			writeOrderJSON(w, http.StatusForbidden, orderErrJSON{Error: "forbidden", Message: err.Error()})
		case errors.Is(err, service.ErrInvalidInput), errors.Is(err, service.ErrNegativeTotal):
			writeOrderJSON(w, http.StatusBadRequest, orderErrJSON{Error: "validation_error", Message: err.Error()})
		default:
			slog.Error("order: criar", "err", err)
			writeOrderJSON(w, http.StatusInternalServerError, orderErrJSON{Error: "internal_error"})
		}
		return
	}

	writeOrderJSON(w, http.StatusCreated, dto.OrderFromDomain(order))
}

// ─── Get — GET /api/v1/orders/{id} ───────────────────────────────────────────

// Get retorna um pedido pelo ID com seus itens.
// Gestor, Financeiro e Vendedor podem ler (escopo de vendedor verificado no service).
func (h *OrderHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeOrderJSON(w, http.StatusBadRequest, orderErrJSON{Error: "missing_id"})
		return
	}

	order, err := h.svc.GetOrder(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrOrderNotFound) {
			writeOrderJSON(w, http.StatusNotFound, orderErrJSON{Error: "not_found"})
			return
		}
		slog.Error("order: buscar", "id", id, "err", err)
		writeOrderJSON(w, http.StatusInternalServerError, orderErrJSON{Error: "internal_error"})
		return
	}

	writeOrderJSON(w, http.StatusOK, dto.OrderFromDomain(order))
}

// ─── Transition — PATCH /api/v1/orders/{id}/status ───────────────────────────

// Transition executa uma transição de status de pedido.
// Apenas Gestor pode transicionar.
// Body: {"status": "confirmado"|"pago"|"cancelado"}
// Resposta 200 + pedido atualizado.
func (h *OrderHandler) Transition(w http.ResponseWriter, r *http.Request) {
	role, _, userID := orderClaimsFromReq(r)
	id := chi.URLParam(r, "id")
	if id == "" {
		writeOrderJSON(w, http.StatusBadRequest, orderErrJSON{Error: "missing_id"})
		return
	}

	var req dto.TransitionOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeOrderJSON(w, http.StatusBadRequest, orderErrJSON{Error: "invalid_json", Message: err.Error()})
		return
	}

	if req.Status == "" {
		writeOrderJSON(w, http.StatusBadRequest, orderErrJSON{Error: "validation_error", Message: "status obrigatório"})
		return
	}

	order, err := h.svc.TransitionOrder(r.Context(), service.TransitionOrderReq{
		OrderID:     id,
		To:          domain.OrderStatus(req.Status),
		ActorUserID: userID,
		ActorRole:   role,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrForbidden):
			writeOrderJSON(w, http.StatusForbidden, orderErrJSON{Error: "forbidden", Message: err.Error()})
		case errors.Is(err, repository.ErrOrderNotFound):
			writeOrderJSON(w, http.StatusNotFound, orderErrJSON{Error: "not_found"})
		case errors.Is(err, domain.ErrInvalidTransition):
			writeOrderJSON(w, http.StatusUnprocessableEntity, orderErrJSON{Error: "invalid_transition", Message: err.Error()})
		case errors.Is(err, service.ErrNoCommissionRule):
			writeOrderJSON(w, http.StatusUnprocessableEntity, orderErrJSON{Error: "no_commission_rule", Message: err.Error()})
		default:
			slog.Error("order: transicionar", "id", id, "to", req.Status, "err", err)
			writeOrderJSON(w, http.StatusInternalServerError, orderErrJSON{Error: "internal_error"})
		}
		return
	}

	writeOrderJSON(w, http.StatusOK, dto.OrderFromDomain(order))
}
