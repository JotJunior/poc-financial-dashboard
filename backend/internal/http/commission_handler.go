// Package http — handlers HTTP de comissões e apuração.
// Task 5.3: POST /commissions/apurate, GET /commissions, PATCH /commissions/{id}/status,
//           GET /commissions/{id}
// RBAC: apurar → apenas Gestor; transicionar → apenas Financeiro; listar/ler → escopo.
// Constitution P-III: values em int64 centavos — zero float nos DTOs.
// Constitution P-IV: RBAC deny-by-default (servidor é a barreira real).
package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"financial-dashboard/backend/internal/domain"
	"financial-dashboard/backend/internal/dto"
	"financial-dashboard/backend/internal/http/middleware"
	"financial-dashboard/backend/internal/repository"
	"financial-dashboard/backend/internal/service"
)

// ─── Interface do service ──────────────────────────────────────────────────────

// CommissionServiceI abstrai ApurationService para facilitar testes.
type CommissionServiceI interface {
	ApurateMonth(ctx interface{ Done() <-chan struct{} }, year, month int, actorRole string) (service.ApurationResult, error)
	ListCommissions(ctx interface{ Done() <-chan struct{} }, filter repository.CommissionFilter, actorRole string, actorVendorID string) ([]repository.Commission, error)
	GetCommission(ctx interface{ Done() <-chan struct{} }, id string, actorRole string, actorVendorID string) (repository.Commission, error)
	TransitionCommission(ctx interface{ Done() <-chan struct{} }, id string, to domain.CommissionStatus, actorID string, actorRole string, motivo string) (repository.Commission, error)
}

// ─── CommissionHandler ────────────────────────────────────────────────────────

// CommissionHandler agrupa os handlers de /commissions/*.
type CommissionHandler struct {
	svc *service.ApurationService
}

// NewCommissionHandler cria um CommissionHandler.
func NewCommissionHandler(svc *service.ApurationService) *CommissionHandler {
	return &CommissionHandler{svc: svc}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func writeCommJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

type commErrJSON struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

func commClaimsFromReq(r *http.Request) (role, vendorID, userID string) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		return "", "", ""
	}
	return claims.Role, claims.VendorID, claims.Subject
}

// ─── POST /api/v1/commissions/apurate ─────────────────────────────────────────

// Apurate executa a apuração mensal de comissões.
// Body: {"year": 2026, "month": 6}
// RBAC: apenas Gestor.
// Retorna ApurationResult com totais calculados e pulados.
func (h *CommissionHandler) Apurate(w http.ResponseWriter, r *http.Request) {
	role, _, _ := commClaimsFromReq(r)

	var req dto.ApurateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCommJSON(w, http.StatusBadRequest, commErrJSON{Error: "bad_request", Message: "body JSON inválido"})
		return
	}

	if req.Year == 0 || req.Month == 0 {
		writeCommJSON(w, http.StatusBadRequest, commErrJSON{Error: "bad_request", Message: "year e month são obrigatórios"})
		return
	}

	result, err := h.svc.ApurateMonth(r.Context(), req.Year, req.Month, role)
	if err != nil {
		slog.Error("apuração falhou", "err", err)
		switch {
		case errors.Is(err, service.ErrForbidden):
			writeCommJSON(w, http.StatusForbidden, commErrJSON{Error: "forbidden", Message: "apenas Gestor pode apurar"})
		case errors.Is(err, service.ErrInvalidInput):
			writeCommJSON(w, http.StatusBadRequest, commErrJSON{Error: "bad_request", Message: err.Error()})
		default:
			writeCommJSON(w, http.StatusInternalServerError, commErrJSON{Error: "internal_error", Message: "falha na apuração"})
		}
		return
	}

	writeCommJSON(w, http.StatusOK, dto.ApurationResultResponse{
		PeriodYear:  result.PeriodYear,
		PeriodMonth: result.PeriodMonth,
		Calculated:  result.Calculated,
		Skipped:     result.Skipped,
		TotalCents:  result.TotalCents,
	})
}

// ─── GET /api/v1/commissions ──────────────────────────────────────────────────

// List retorna comissões com filtros opcionais.
// Query params: ?vendorId=, ?year=, ?month=, ?status=
// RBAC: Gestor e Financeiro veem todas; Vendedor vê apenas as suas.
func (h *CommissionHandler) List(w http.ResponseWriter, r *http.Request) {
	role, vendorID, _ := commClaimsFromReq(r)

	filter := repository.CommissionFilter{}

	// Filtros opcionais via query params (bind params no repository — CHK020).
	if v := r.URL.Query().Get("vendorId"); v != "" {
		filter.VendorID = &v
	}
	if v := r.URL.Query().Get("year"); v != "" {
		yr, err := strconv.Atoi(v)
		if err != nil {
			writeCommJSON(w, http.StatusBadRequest, commErrJSON{Error: "bad_request", Message: "year inválido"})
			return
		}
		filter.PeriodYear = &yr
	}
	if v := r.URL.Query().Get("month"); v != "" {
		mo, err := strconv.Atoi(v)
		if err != nil {
			writeCommJSON(w, http.StatusBadRequest, commErrJSON{Error: "bad_request", Message: "month inválido"})
			return
		}
		filter.PeriodMonth = &mo
	}
	if v := r.URL.Query().Get("status"); v != "" {
		s := domain.CommissionStatus(v)
		filter.Status = &s
	}

	commissions, err := h.svc.ListCommissions(r.Context(), filter, role, vendorID)
	if err != nil {
		if errors.Is(err, service.ErrForbidden) {
			writeCommJSON(w, http.StatusForbidden, commErrJSON{Error: "forbidden"})
			return
		}
		slog.Error("listar comissões falhou", "err", err)
		writeCommJSON(w, http.StatusInternalServerError, commErrJSON{Error: "internal_error"})
		return
	}

	resp := make([]dto.CommissionResponse, 0, len(commissions))
	for _, c := range commissions {
		resp = append(resp, dto.CommissionFromDomain(c))
	}
	writeCommJSON(w, http.StatusOK, resp)
}

// ─── GET /api/v1/commissions/{id} ─────────────────────────────────────────────

// Get retorna uma comissão pelo ID.
// RBAC: Gestor e Financeiro veem qualquer; Vendedor vê apenas as suas.
func (h *CommissionHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	role, vendorID, _ := commClaimsFromReq(r)

	c, err := h.svc.GetCommission(r.Context(), id, role, vendorID)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrCommissionNotFound):
			writeCommJSON(w, http.StatusNotFound, commErrJSON{Error: "not_found"})
		case errors.Is(err, service.ErrForbidden):
			writeCommJSON(w, http.StatusForbidden, commErrJSON{Error: "forbidden"})
		default:
			slog.Error("buscar comissão falhou", "id", id, "err", err)
			writeCommJSON(w, http.StatusInternalServerError, commErrJSON{Error: "internal_error"})
		}
		return
	}

	writeCommJSON(w, http.StatusOK, dto.CommissionFromDomain(c))
}

// ─── PATCH /api/v1/commissions/{id}/status ────────────────────────────────────

// Transition executa transição de status de uma comissão.
// Body: {"status": "aprovado", "motivo": "..."}
// RBAC: apenas Financeiro.
// Transições válidas: pendente→aprovado, aprovado→pago, aprovado→pendente (com motivo).
func (h *CommissionHandler) Transition(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	role, _, userID := commClaimsFromReq(r)

	var req dto.TransitionCommissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCommJSON(w, http.StatusBadRequest, commErrJSON{Error: "bad_request", Message: "body JSON inválido"})
		return
	}

	if req.Status == "" {
		writeCommJSON(w, http.StatusBadRequest, commErrJSON{Error: "bad_request", Message: "status é obrigatório"})
		return
	}

	to := domain.CommissionStatus(req.Status)

	c, err := h.svc.TransitionCommission(r.Context(), id, to, userID, role, req.Motivo)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrForbidden):
			writeCommJSON(w, http.StatusForbidden, commErrJSON{Error: "forbidden", Message: "apenas Financeiro pode aprovar/pagar comissões"})
		case errors.Is(err, repository.ErrCommissionNotFound):
			writeCommJSON(w, http.StatusNotFound, commErrJSON{Error: "not_found"})
		case errors.Is(err, domain.ErrInvalidTransition):
			writeCommJSON(w, http.StatusUnprocessableEntity, commErrJSON{Error: "invalid_transition", Message: err.Error()})
		case errors.Is(err, domain.ErrMissingReason):
			writeCommJSON(w, http.StatusBadRequest, commErrJSON{Error: "bad_request", Message: "motivo obrigatório para aprovado→pendente"})
		default:
			slog.Error("transição de comissão falhou", "id", id, "err", err)
			writeCommJSON(w, http.StatusInternalServerError, commErrJSON{Error: "internal_error"})
		}
		return
	}

	writeCommJSON(w, http.StatusOK, dto.CommissionFromDomain(c))
}

// RegisterRoutes registra as rotas de comissões no router chi.
// Chamado por main.go ao montar o router.
func (h *CommissionHandler) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler, requireRole func(...string) func(http.Handler) http.Handler) {
	r.Route("/commissions", func(r chi.Router) {
		r.Use(authMiddleware)

		// POST /apurate — apenas Gestor
		r.With(requireRole("gestor")).Post("/apurate", h.Apurate)

		// GET / — Gestor, Financeiro, Vendedor (com escopo)
		r.With(requireRole("gestor", "financeiro", "vendedor")).Get("/", h.List)

		// GET /{id} — Gestor, Financeiro, Vendedor (com escopo)
		r.With(requireRole("gestor", "financeiro", "vendedor")).Get("/{id}", h.Get)

		// PATCH /{id}/status — apenas Financeiro
		r.With(requireRole("financeiro")).Patch("/{id}/status", h.Transition)
	})
}
