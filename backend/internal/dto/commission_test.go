package dto

import (
	"regexp"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"financial-dashboard/backend/internal/domain"
	"financial-dashboard/backend/internal/repository"
)

// Regressão: appliedPercentage deve ser serializado com 4 casas decimais fixas
// ("10.0000"), conforme o contrato (NUMERIC(7,4)) e o regex Zod do frontend
// (/^\d+\.\d{4}$/). decimal.Decimal.String() removia zeros à direita ("10"),
// fazendo a validação do frontend rejeitar a resposta e quebrar toda a página
// de Comissões. StringFixed(4) corrige. Ver bugfix (commissions Vendedor).
func TestCommissionFromDomain_AppliedPercentageHas4Decimals(t *testing.T) {
	zodRegex := regexp.MustCompile(`^\d+\.\d{4}$`)
	cases := []struct {
		in   string
		want string
	}{
		{"10", "10.0000"},
		{"5.5", "5.5000"},
		{"8", "8.0000"},
		{"12.3456", "12.3456"},
	}
	for _, c := range cases {
		pct, err := decimal.NewFromString(c.in)
		if err != nil {
			t.Fatalf("decimal inválido %q: %v", c.in, err)
		}
		resp := CommissionFromDomain(repository.Commission{
			ID:                "id",
			OrderID:           "ord",
			VendorID:          "ven",
			AppliedPercentage: pct,
			RuleID:            "rule",
			Status:            domain.CommissionStatus("pendente"),
			CalculatedAt:      time.Unix(0, 0).UTC(),
		})
		if resp.AppliedPercentage != c.want {
			t.Errorf("in=%s: appliedPercentage=%q, esperado %q", c.in, resp.AppliedPercentage, c.want)
		}
		if !zodRegex.MatchString(resp.AppliedPercentage) {
			t.Errorf("in=%s: %q não casa com o regex Zod do frontend /^\\d+\\.\\d{4}$/", c.in, resp.AppliedPercentage)
		}
	}
}
