package domain

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// Money representa um valor monetário em centavos (int64).
// Constitution P-III NON-NEGOTIABLE: zero ponto flutuante em qualquer
// ponto intermediário de cálculo monetário.
type Money int64

// MoneyFromCents converte centavos int64 para Money.
func MoneyFromCents(cents int64) Money {
	return Money(cents)
}

// ToCents retorna o valor em centavos como int64.
func (m Money) ToCents() int64 {
	return int64(m)
}

// String formata o valor como "R$ X,XX" para exibição.
func (m Money) String() string {
	cents := int64(m)
	negative := cents < 0
	if negative {
		cents = -cents
	}
	reais := cents / 100
	centavos := cents % 100
	s := fmt.Sprintf("R$ %d,%02d", reais, centavos)
	if negative {
		s = "-" + s
	}
	return s
}

// RoundCommission calcula a comissão sobre orderCents com o percentual dado,
// usando arredondamento half-up (shopspring/decimal).
// É o ÚNICO ponto de arredondamento monetário no sistema.
// Nunca usa float em nenhum ponto intermediário.
//
// Exemplo: RoundCommission(500000, decimal.NewFromInt(8)) == 40000
// (8% de R$5.000,00 = R$400,00 = 40000 centavos)
func RoundCommission(orderCents int64, percentage decimal.Decimal) int64 {
	// Usando shopspring/decimal para precisão exata — zero float
	order := decimal.NewFromInt(orderCents)
	hundred := decimal.NewFromInt(100)

	// commission = orderCents * percentage / 100, arredondado half-up (0 casas decimais)
	// decimal.Round(0) usa half-up por padrão (arredonda .5 para cima)
	commission := order.Mul(percentage).Div(hundred).Round(0)

	return commission.IntPart()
}
