package domain

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMoneyFromCents verifica a conversão de centavos para Money.
func TestMoneyFromCents(t *testing.T) {
	m := MoneyFromCents(50000)
	assert.Equal(t, int64(50000), m.ToCents())
}

// TestMoneyString verifica a formatação "R$ X,XX".
func TestMoneyString(t *testing.T) {
	tests := []struct {
		cents    int64
		expected string
	}{
		{0, "R$ 0,00"},
		{100, "R$ 1,00"},
		{50000, "R$ 500,00"},
		{500000, "R$ 5000,00"},
		{100000000, "R$ 1000000,00"},
		{1, "R$ 0,01"},
		{99, "R$ 0,99"},
		{-100, "-R$ 1,00"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			m := MoneyFromCents(tt.cents)
			assert.Equal(t, tt.expected, m.String())
		})
	}
}

// TestRoundCommission_CasosCanonicos verifica os dois casos canônicos do SC-002.
func TestRoundCommission_CasosCanonicos(t *testing.T) {
	// Caso 1: 8% de R$5.000,00 = R$400,00 exatos (spec SC-002)
	result1 := RoundCommission(500000, decimal.NewFromInt(8))
	assert.Equal(t, int64(40000), result1,
		"8%% de R$5.000,00 deve ser R$400,00 (40000 centavos)")

	// Caso 2: 10% de R$1.000,00 = R$100,00 exatos
	result2 := RoundCommission(100000, decimal.NewFromInt(10))
	assert.Equal(t, int64(10000), result2,
		"10%% de R$1.000,00 deve ser R$100,00 (10000 centavos)")
}

// TestRoundCommission_Idempotencia verifica SC-002: resultado idêntico em 100% das execuções.
func TestRoundCommission_Idempotencia(t *testing.T) {
	orderCents := int64(500000)
	pct := decimal.NewFromInt(8)
	expected := RoundCommission(orderCents, pct)

	for i := 0; i < 1000; i++ {
		result := RoundCommission(orderCents, pct)
		assert.Equal(t, expected, result,
			"RoundCommission deve ser determinístico — falhou na iteração %d", i)
	}
}

// TestRoundCommission_HalfUp verifica que o arredondamento é half-up.
// 1/3 de 1 centavo = 0,333... → arredondado para 0
// 2/3 de 1 centavo = 0,666... → arredondado para 1
func TestRoundCommission_HalfUp(t *testing.T) {
	tests := []struct {
		name       string
		cents      int64
		percentage string // string para evitar float na definição do teste
		expected   int64
	}{
		// 5% de 1 centavo = 0.05 → arredonda para 0 (< 0.5)
		{"5% de 1ct", 1, "5", 0},
		// 50% de 1 centavo = 0.5 → arredonda para 1 (half-up)
		{"50% de 1ct half-up", 1, "50", 1},
		// 33.333...% de 100 = 33.333... → 33
		{"33.3333% de 100ct", 100, "33.3333", 33},
		// 66.6666% de 100 = 66.6666 → 67
		{"66.6666% de 100ct", 100, "66.6666", 67},
		// Percentual fracionário: 5.5% de 10000 = 550.0 → 550
		{"5.5% de R$100,00", 10000, "5.5", 550},
		// 8% de 500000 = 40000 exatos
		{"8% de R$5000,00", 500000, "8", 40000},
		// 10% de 100000 = 10000 exatos
		{"10% de R$1000,00", 100000, "10", 10000},
		// Zero pedido → zero comissão
		{"0% de qualquer", 500000, "0", 0},
		{"qualquer% de 0", 0, "8", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pct, err := decimal.NewFromString(tt.percentage)
			require.NoError(t, err)
			result := RoundCommission(tt.cents, pct)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestRoundCommission_SemFloat verifica que money.go não usa float em nenhum ponto.
// Propriedade estática: parseia o AST do arquivo money.go e rejeita qualquer
// BasicLit de tipo FLOAT ou identificador float32/float64.
func TestRoundCommission_SemFloat(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "money.go", nil, 0)
	require.NoError(t, err, "falha ao parsear money.go — arquivo não encontrado?")

	var floatNodes []string

	ast.Inspect(f, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.BasicLit:
			if v.Kind == token.FLOAT {
				floatNodes = append(floatNodes, fmt.Sprintf("literal float em %s: %s",
					fset.Position(v.Pos()), v.Value))
			}
		case *ast.Ident:
			if v.Name == "float32" || v.Name == "float64" {
				floatNodes = append(floatNodes, fmt.Sprintf("tipo float em %s: %s",
					fset.Position(v.Pos()), v.Name))
			}
		}
		return true
	})

	assert.Empty(t, floatNodes,
		"money.go NÃO deve conter floats (Constitution P-III): %v", floatNodes)
}
