// Package http — handlers HTTP de dashboard e métricas.
// Task 6.2: GET /dashboard/consolidated, /dashboard/vendor, /dashboard/commissions/pending,
//           GET /orders/{id}/drilldown.
// RBAC (P-IV):
//   - /consolidated:  gestor, financeiro
//   - /vendor:        gestor (query ?vendorId), financeiro, vendedor (próprio)
//   - /commissions/pending: gestor, financeiro
//   - /orders/{id}/drilldown: todos (com escopo de vendedor no service)
// Constitution P-III: valores em centavos — zero float nos DTOs.
package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"financial-dashboard/backend/internal/dto"
	"financial-dashboard/backend/internal/http/middleware"
	"financial-dashboard/backend/internal/repository"
	"financial-dashboard/backend/internal/service"
)

// ─── DashboardHandler ────────────────────────────────────────────────────────

// DashboardHandler agrupa os handlers de /dashboard/* e drilldown.
type DashboardHandler struct {
	svc *service.DashboardService
}

// NewDashboardHandler cria um DashboardHandler.
func NewDashboardHandler(svc *service.DashboardService) *DashboardHandler {
	return &DashboardHandler{svc: svc}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func writeDashJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

type dashErrJSON struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

func dashClaimsFromReq(r *http.Request) (role, vendorID, userID string) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		return "", "", ""
	}
	return claims.Role, claims.VendorID, claims.Subject
}

// parseDashboardFilter extrai year, month, startDate, endDate de query params.
// Ignora parâmetros ausentes ou inválidos (filtro permissivo).
func parseDashboardFilter(r *http.Request) repository.DashboardFilter {
	q := r.URL.Query()
	var f repository.DashboardFilter

	if y := q.Get("year"); y != "" {
		if v, err := strconv.Atoi(y); err == nil && v > 0 {
			f.Year = v
		}
	}
	if m := q.Get("month"); m != "" {
		if v, err := strconv.Atoi(m); err == nil && v >= 1 && v <= 12 {
			f.Month = v
		}
	}
	return f
}

// ─── GET /api/v1/dashboard/consolidated ──────────────────────────────────────

// Consolidated retorna métricas agregadas de todos os vendedores.
// Task 6.2.2 — apenas Gestor e Financeiro.
// Query params: ?year=2026&month=5
func (h *DashboardHandler) Consolidated(w http.ResponseWriter, r *http.Request) {
	role, _, _ := dashClaimsFromReq(r)
	filter := parseDashboardFilter(r)

	dash, err := h.svc.GetConsolidated(r.Context(), filter, role)
	if err != nil {
		if errors.Is(err, service.ErrForbidden) {
			writeDashJSON(w, http.StatusForbidden, dashErrJSON{Error: "forbidden", Message: err.Error()})
			return
		}
		writeDashJSON(w, http.StatusInternalServerError, dashErrJSON{Error: "internal_error", Message: err.Error()})
		return
	}

	writeDashJSON(w, http.StatusOK, dto.ConsolidatedDashboardFromDomain(dash))
}

// ─── GET /api/v1/dashboard/vendor ────────────────────────────────────────────

// VendorDashboard retorna métricas de vendedor específico.
// Task 6.2.3.
// Query params: ?vendorId=<uuid>&year=2026&month=5
// Vendedor: ignora vendorId (usa o próprio do JWT). Gestor/Financeiro: vendorId obrigatório.
func (h *DashboardHandler) VendorDashboard(w http.ResponseWriter, r *http.Request) {
	role, vendorID, _ := dashClaimsFromReq(r)
	filter := parseDashboardFilter(r)

	requestedVendorID := r.URL.Query().Get("vendorId")

	dash, err := h.svc.GetVendorDashboard(r.Context(), requestedVendorID, filter, role, vendorID)
	if err != nil {
		if errors.Is(err, service.ErrForbidden) {
			writeDashJSON(w, http.StatusForbidden, dashErrJSON{Error: "forbidden", Message: err.Error()})
			return
		}
		writeDashJSON(w, http.StatusBadRequest, dashErrJSON{Error: "bad_request", Message: err.Error()})
		return
	}

	writeDashJSON(w, http.StatusOK, dto.VendorDashboardFromDomain(dash))
}

// ─── GET /api/v1/dashboard/commissions/pending ───────────────────────────────

// PendingCommissions retorna indicadores de comissões pendentes de aprovação/pagamento.
// Task 6.2.4 — apenas Gestor e Financeiro (FR-019).
func (h *DashboardHandler) PendingCommissions(w http.ResponseWriter, r *http.Request) {
	role, _, _ := dashClaimsFromReq(r)

	summary, err := h.svc.GetPendingCommissions(r.Context(), role)
	if err != nil {
		if errors.Is(err, service.ErrForbidden) {
			writeDashJSON(w, http.StatusForbidden, dashErrJSON{Error: "forbidden", Message: err.Error()})
			return
		}
		writeDashJSON(w, http.StatusInternalServerError, dashErrJSON{Error: "internal_error", Message: err.Error()})
		return
	}

	writeDashJSON(w, http.StatusOK, dto.PendingCommissionsFromDomain(summary))
}

// ─── GET /api/v1/orders/{id}/drilldown ───────────────────────────────────────

// DrillDown retorna rastreabilidade completa: pedido → comissões → estornos.
// Task 6.2.5 — SC-004; todos os papéis autenticados (escopo de vendedor no service).
func (h *DashboardHandler) DrillDown(w http.ResponseWriter, r *http.Request) {
	role, vendorID, _ := dashClaimsFromReq(r)
	orderID := chi.URLParam(r, "id")

	dd, err := h.svc.GetDrillDown(r.Context(), orderID, role, vendorID)
	if err != nil {
		if errors.Is(err, service.ErrForbidden) {
			writeDashJSON(w, http.StatusForbidden, dashErrJSON{Error: "forbidden", Message: err.Error()})
			return
		}
		writeDashJSON(w, http.StatusNotFound, dashErrJSON{Error: "not_found", Message: err.Error()})
		return
	}

	writeDashJSON(w, http.StatusOK, dto.DrillDownFromDomain(dd))
}
