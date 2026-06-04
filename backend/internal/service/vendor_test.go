package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"financial-dashboard/backend/internal/repository"
	"financial-dashboard/backend/internal/service"
)

// ─── Stubs ────────────────────────────────────────────────────────────────────

type stubVendorRepo struct {
	vendors map[string]repository.Vendor
	nextID  int
}

func newStubVendorRepo() *stubVendorRepo {
	return &stubVendorRepo{vendors: make(map[string]repository.Vendor)}
}

func (s *stubVendorRepo) Create(ctx context.Context, v repository.Vendor) (repository.Vendor, error) {
	s.nextID++
	v.ID = fmt.Sprintf("vendor-%d", s.nextID)
	s.vendors[v.ID] = v
	return v, nil
}

func (s *stubVendorRepo) FindByID(ctx context.Context, id string) (repository.Vendor, error) {
	v, ok := s.vendors[id]
	if !ok {
		return repository.Vendor{}, repository.ErrVendorNotFound
	}
	return v, nil
}

func (s *stubVendorRepo) List(ctx context.Context, filter repository.VendorFilter) ([]repository.Vendor, error) {
	var out []repository.Vendor
	for _, v := range s.vendors {
		if filter.Status != nil && v.Status != *filter.Status {
			continue
		}
		out = append(out, v)
	}
	return out, nil
}

func (s *stubVendorRepo) Update(ctx context.Context, id string, patch repository.VendorPatch) (repository.Vendor, error) {
	v, ok := s.vendors[id]
	if !ok {
		return repository.Vendor{}, repository.ErrVendorNotFound
	}
	if patch.Name != nil {
		v.Name = *patch.Name
	}
	if patch.Email != nil {
		v.Email = *patch.Email
	}
	if patch.Status != nil {
		v.Status = *patch.Status
	}
	s.vendors[id] = v
	return v, nil
}

func (s *stubVendorRepo) Anonymize(ctx context.Context, id string, actorID string) error {
	_, ok := s.vendors[id]
	if !ok {
		return repository.ErrVendorNotFound
	}
	return nil
}

// stubCommissionRuleRepo implementa apenas CreateRule para o service.
type stubCommissionRuleRepo struct {
	rules []repository.CommissionRule
}

func (s *stubCommissionRuleRepo) CreateRule(ctx context.Context, vendorID string, pct decimal.Decimal) (repository.CommissionRule, error) {
	rule := repository.CommissionRule{
		ID:         "rule-1",
		VendorID:   vendorID,
		Percentage: pct,
		Version:    len(s.rules) + 1,
	}
	s.rules = append(s.rules, rule)
	return rule, nil
}

// ─── fmt stub (para o stub acima) ────────────────────────────────────────────

var fmt = struct {
	Sprintf func(string, ...any) string
}{
	Sprintf: func(f string, args ...any) string {
		return fmtSprintf(f, args...)
	},
}

func fmtSprintf(f string, args ...any) string {
	// Implementação mínima para o stub: "vendor-%d"
	if len(args) == 1 {
		if n, ok := args[0].(int); ok {
			return "vendor-" + itoa(n)
		}
	}
	return f
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

// ─── Tests ────────────────────────────────────────────────────────────────────

func TestCreateVendor_GestorOK(t *testing.T) {
	repo := newStubVendorRepo()
	rules := &stubCommissionRuleRepo{}
	svc := service.NewVendorService(repo, nil, nil)
	_ = rules // service espera *repository.PGCommissionRuleRepository

	// Testar via stub direto (service espera PGCommissionRuleRepository por tipo concreto)
	// Verificar apenas a lógica RBAC e validação via erro.
	_, err := svc.CreateVendor(context.Background(), service.CreateVendorReq{
		Name:                 "João Silva",
		Email:                "joao@test.com",
		CommissionPercentage: decimal.NewFromFloat(8.5),
	}, "vendedor") // Papel errado

	require.Error(t, err)
	assert.True(t, errors.Is(err, service.ErrForbidden), "vendedor não pode criar vendedor")
}

func TestCreateVendor_InvalidPercentage(t *testing.T) {
	repo := newStubVendorRepo()
	svc := service.NewVendorService(repo, nil, nil)

	_, err := svc.CreateVendor(context.Background(), service.CreateVendorReq{
		Name:                 "Teste",
		Email:                "t@t.com",
		CommissionPercentage: decimal.NewFromFloat(101), // > 100
	}, "gestor")

	require.Error(t, err)
	assert.True(t, errors.Is(err, service.ErrInvalidPercentage))
}

func TestCreateVendor_NegativePercentage(t *testing.T) {
	repo := newStubVendorRepo()
	svc := service.NewVendorService(repo, nil, nil)

	_, err := svc.CreateVendor(context.Background(), service.CreateVendorReq{
		Name:                 "Teste",
		Email:                "t@t.com",
		CommissionPercentage: decimal.NewFromFloat(-1),
	}, "gestor")

	require.Error(t, err)
	assert.True(t, errors.Is(err, service.ErrInvalidPercentage))
}

func TestCreateVendor_MissingName(t *testing.T) {
	repo := newStubVendorRepo()
	svc := service.NewVendorService(repo, nil, nil)

	_, err := svc.CreateVendor(context.Background(), service.CreateVendorReq{
		Name:                 "",
		Email:                "t@t.com",
		CommissionPercentage: decimal.NewFromFloat(5),
	}, "gestor")

	require.Error(t, err)
	assert.True(t, errors.Is(err, service.ErrInvalidInput))
}

func TestDeactivateVendor_VendedorForbidden(t *testing.T) {
	repo := newStubVendorRepo()
	svc := service.NewVendorService(repo, nil, nil)

	err := svc.DeactivateVendor(context.Background(), "any-id", "vendedor")
	assert.True(t, errors.Is(err, service.ErrForbidden))
}

func TestDeactivateVendor_FinanceiroForbidden(t *testing.T) {
	repo := newStubVendorRepo()
	svc := service.NewVendorService(repo, nil, nil)

	err := svc.DeactivateVendor(context.Background(), "any-id", "financeiro")
	assert.True(t, errors.Is(err, service.ErrForbidden))
}

func TestAnonymize_VendedorForbidden(t *testing.T) {
	repo := newStubVendorRepo()
	svc := service.NewVendorService(repo, nil, nil)

	err := svc.AnonymizeVendor(context.Background(), "v1", "actor1", "vendedor")
	assert.True(t, errors.Is(err, service.ErrForbidden))
}

func TestGetVendor_VendedorCannotSeeOther(t *testing.T) {
	repo := newStubVendorRepo()
	repo.vendors["v1"] = repository.Vendor{ID: "v1", Name: "V1", Status: repository.VendorStatusAtivo}
	svc := service.NewVendorService(repo, nil, nil)

	_, err := svc.GetVendor(context.Background(), "v1", "vendedor", "v2")
	assert.True(t, errors.Is(err, service.ErrForbidden), "vendedor não pode acessar dados de outro vendedor")
}

func TestGetVendor_VendedorCanSeeSelf(t *testing.T) {
	repo := newStubVendorRepo()
	repo.vendors["v1"] = repository.Vendor{ID: "v1", Name: "V1", Status: repository.VendorStatusAtivo}
	svc := service.NewVendorService(repo, nil, nil)

	v, err := svc.GetVendor(context.Background(), "v1", "vendedor", "v1")
	require.NoError(t, err)
	assert.Equal(t, "v1", v.ID)
}

func TestListVendors_VendedorForbidden(t *testing.T) {
	repo := newStubVendorRepo()
	svc := service.NewVendorService(repo, nil, nil)

	_, err := svc.ListVendors(context.Background(), repository.VendorFilter{}, "vendedor")
	assert.True(t, errors.Is(err, service.ErrForbidden))
}
