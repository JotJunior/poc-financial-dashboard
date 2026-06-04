// Package http — handlers HTTP de vendedores.
// Task 3.4: GET/POST /api/v1/vendors, GET/PATCH/DELETE /api/v1/vendors/{id}
// RBAC: RequireAuth + RequireRole conforme constitution P-IV.
// CHK020: bind params em todos os repositórios — zero interpolação SQL.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"

	"financial-dashboard/backend/internal/dto"
	"financial-dashboard/backend/internal/http/middleware"
	"financial-dashboard/backend/internal/repository"
	"financial-dashboard/backend/internal/service"
)

// ─── Interface do service ──────────────────────────────────────────────────────

// VendorServiceI abstrai VendorService para facilitar testes.
type VendorServiceI interface {
	CreateVendor(ctx context.Context, req service.CreateVendorReq, actorRole string) (repository.Vendor, error)
	GetVendor(ctx context.Context, id string, actorRole string, actorVendorID string) (repository.Vendor, error)
	ListVendors(ctx context.Context, filter repository.VendorFilter, actorRole string) ([]repository.Vendor, error)
	UpdateVendor(ctx context.Context, id string, req service.UpdateVendorReq, actorRole string) (repository.Vendor, error)
	DeactivateVendor(ctx context.Context, id string, actorRole string) error
	AnonymizeVendor(ctx context.Context, id string, actorUserID string, actorRole string) error
}

// ─── VendorHandler ────────────────────────────────────────────────────────────

// VendorHandler agrupa os handlers de /vendors/*.
type VendorHandler struct {
	svc VendorServiceI
}

// NewVendorHandler cria um VendorHandler.
func NewVendorHandler(svc VendorServiceI) *VendorHandler {
	return &VendorHandler{svc: svc}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func writeVendorJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

type vendorErrJSON struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

func claimsFromReq(r *http.Request) (role, vendorID, userID string) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		return "", "", ""
	}
	return claims.Role, claims.VendorID, claims.Subject
}

// ─── List — GET /api/v1/vendors ───────────────────────────────────────────────

// List godoc: retorna lista de vendedores. Gestor e Financeiro.
// Query param: ?status=ativo|inativo
func (h *VendorHandler) List(w http.ResponseWriter, r *http.Request) {
	role, _, _ := claimsFromReq(r)

	filter := repository.VendorFilter{}
	if s := r.URL.Query().Get("status"); s != "" {
		v := repository.VendorStatus(s)
		filter.Status = &v
	}

	vendors, err := h.svc.ListVendors(r.Context(), filter, role)
	if err != nil {
		if errors.Is(err, service.ErrForbidden) {
			writeVendorJSON(w, http.StatusForbidden, vendorErrJSON{Error: "forbidden"})
			return
		}
		slog.Error("vendor: listar", "err", err)
		writeVendorJSON(w, http.StatusInternalServerError, vendorErrJSON{Error: "internal_error"})
		return
	}

	resp := make([]dto.VendorResponse, len(vendors))
	for i, v := range vendors {
		resp[i] = dto.VendorFromDomain(v)
	}
	writeVendorJSON(w, http.StatusOK, resp)
}

// ─── Create — POST /api/v1/vendors ────────────────────────────────────────────

// Create godoc: cria vendedor + regra de comissão inicial. Apenas Gestor.
func (h *VendorHandler) Create(w http.ResponseWriter, r *http.Request) {
	role, _, _ := claimsFromReq(r)

	var req dto.CreateVendorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeVendorJSON(w, http.StatusBadRequest, vendorErrJSON{Error: "invalid_request"})
		return
	}

	pct, err := decimal.NewFromString(req.CommissionPercentage)
	if err != nil {
		writeVendorJSON(w, http.StatusBadRequest, vendorErrJSON{
			Error:   "invalid_request",
			Message: "commissionPercentage deve ser um número decimal (ex: '8.5000')",
		})
		return
	}

	vendor, err := h.svc.CreateVendor(r.Context(), service.CreateVendorReq{
		Name:                 req.Name,
		Email:                req.Email,
		CommissionPercentage: pct,
	}, role)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrForbidden):
			writeVendorJSON(w, http.StatusForbidden, vendorErrJSON{Error: "forbidden"})
		case errors.Is(err, service.ErrInvalidInput):
			writeVendorJSON(w, http.StatusBadRequest, vendorErrJSON{Error: "invalid_input", Message: err.Error()})
		case errors.Is(err, service.ErrInvalidPercentage):
			writeVendorJSON(w, http.StatusBadRequest, vendorErrJSON{Error: "invalid_percentage", Message: err.Error()})
		case errors.Is(err, repository.ErrVendorEmailConflict):
			writeVendorJSON(w, http.StatusConflict, vendorErrJSON{Error: "email_conflict", Message: "Email já cadastrado"})
		default:
			slog.Error("vendor: criar", "err", err)
			writeVendorJSON(w, http.StatusInternalServerError, vendorErrJSON{Error: "internal_error"})
		}
		return
	}

	writeVendorJSON(w, http.StatusCreated, dto.VendorFromDomain(vendor))
}

// ─── Get — GET /api/v1/vendors/{id} ──────────────────────────────────────────

// Get godoc: retorna vendedor por ID.
// Gestor e Financeiro veem qualquer vendedor; Vendedor apenas a si mesmo (SC-005).
func (h *VendorHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	role, vendorID, _ := claimsFromReq(r)

	vendor, err := h.svc.GetVendor(r.Context(), id, role, vendorID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrForbidden):
			writeVendorJSON(w, http.StatusForbidden, vendorErrJSON{Error: "forbidden"})
		case errors.Is(err, repository.ErrVendorNotFound):
			writeVendorJSON(w, http.StatusNotFound, vendorErrJSON{Error: "not_found"})
		default:
			slog.Error("vendor: buscar", "err", err, "id", id)
			writeVendorJSON(w, http.StatusInternalServerError, vendorErrJSON{Error: "internal_error"})
		}
		return
	}

	writeVendorJSON(w, http.StatusOK, dto.VendorFromDomain(vendor))
}

// ─── Update — PATCH /api/v1/vendors/{id} ─────────────────────────────────────

// Update godoc: atualiza parcialmente o vendedor. Apenas Gestor.
func (h *VendorHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	role, _, _ := claimsFromReq(r)

	var req dto.UpdateVendorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeVendorJSON(w, http.StatusBadRequest, vendorErrJSON{Error: "invalid_request"})
		return
	}

	updateReq := service.UpdateVendorReq{
		Name:  req.Name,
		Email: req.Email,
	}
	if req.Status != nil {
		v := repository.VendorStatus(*req.Status)
		updateReq.Status = &v
	}
	if req.Percentage != nil {
		pct, err := decimal.NewFromString(*req.Percentage)
		if err != nil {
			writeVendorJSON(w, http.StatusBadRequest, vendorErrJSON{
				Error:   "invalid_request",
				Message: "commissionPercentage deve ser um número decimal",
			})
			return
		}
		updateReq.Percentage = &pct
	}

	vendor, err := h.svc.UpdateVendor(r.Context(), id, updateReq, role)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrForbidden):
			writeVendorJSON(w, http.StatusForbidden, vendorErrJSON{Error: "forbidden"})
		case errors.Is(err, service.ErrInvalidInput):
			writeVendorJSON(w, http.StatusBadRequest, vendorErrJSON{Error: "invalid_input", Message: err.Error()})
		case errors.Is(err, service.ErrInvalidPercentage):
			writeVendorJSON(w, http.StatusBadRequest, vendorErrJSON{Error: "invalid_percentage", Message: err.Error()})
		case errors.Is(err, repository.ErrVendorNotFound):
			writeVendorJSON(w, http.StatusNotFound, vendorErrJSON{Error: "not_found"})
		case errors.Is(err, repository.ErrVendorEmailConflict):
			writeVendorJSON(w, http.StatusConflict, vendorErrJSON{Error: "email_conflict"})
		default:
			slog.Error("vendor: atualizar", "err", err, "id", id)
			writeVendorJSON(w, http.StatusInternalServerError, vendorErrJSON{Error: "internal_error"})
		}
		return
	}

	writeVendorJSON(w, http.StatusOK, dto.VendorFromDomain(vendor))
}

// ─── Delete — DELETE /api/v1/vendors/{id} (anonimização LGPD) ─────────────────

// Delete godoc: anonimiza PII do vendedor (LGPD). Apenas Gestor. Body: {"confirm":true}.
// CHK077: requer confirm=true para evitar exclusão acidental.
func (h *VendorHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	role, _, userID := claimsFromReq(r)

	var req dto.UpdateVendorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeVendorJSON(w, http.StatusBadRequest, vendorErrJSON{Error: "invalid_request"})
		return
	}

	if req.Confirm == nil || !*req.Confirm {
		writeVendorJSON(w, http.StatusBadRequest, vendorErrJSON{
			Error:   "confirmation_required",
			Message: "Informe {\"confirm\":true} para confirmar a anonimização",
		})
		return
	}

	if err := h.svc.AnonymizeVendor(r.Context(), id, userID, role); err != nil {
		switch {
		case errors.Is(err, service.ErrForbidden):
			writeVendorJSON(w, http.StatusForbidden, vendorErrJSON{Error: "forbidden"})
		case errors.Is(err, repository.ErrVendorNotFound):
			writeVendorJSON(w, http.StatusNotFound, vendorErrJSON{Error: "not_found"})
		default:
			slog.Error("vendor: anonimizar", "err", err, "id", id)
			writeVendorJSON(w, http.StatusInternalServerError, vendorErrJSON{Error: "internal_error"})
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
