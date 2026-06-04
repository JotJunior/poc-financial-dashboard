package domain

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOrderTransition_TransicoesValidas verifica as 4 transições permitidas (FR-008).
func TestOrderTransition_TransicoesValidas(t *testing.T) {
	tests := []struct {
		name                  string
		from                  OrderStatus
		to                    OrderStatus
		requiresReversalCheck bool
	}{
		{
			name:                  "rascunho→confirmado",
			from:                  OrderStatusRascunho,
			to:                    OrderStatusConfirmado,
			requiresReversalCheck: false,
		},
		{
			name:                  "confirmado→pago",
			from:                  OrderStatusConfirmado,
			to:                    OrderStatusPago,
			requiresReversalCheck: false,
		},
		{
			name:                  "confirmado→cancelado",
			from:                  OrderStatusConfirmado,
			to:                    OrderStatusCancelado,
			requiresReversalCheck: false,
		},
		{
			name:                  "pago→cancelado (RequiresReversalCheck=true)",
			from:                  OrderStatusPago,
			to:                    OrderStatusCancelado,
			requiresReversalCheck: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.from.Transition(tt.to)
			require.NoError(t, err, "transição válida não deve retornar erro")
			assert.Equal(t, tt.from, result.From)
			assert.Equal(t, tt.to, result.To)
			assert.Equal(t, tt.requiresReversalCheck, result.RequiresReversalCheck)
		})
	}
}

// TestOrderTransition_TransicoesInvalidas verifica que transições não permitidas
// retornam ErrInvalidTransition (FR-008).
func TestOrderTransition_TransicoesInvalidas(t *testing.T) {
	tests := []struct {
		name string
		from OrderStatus
		to   OrderStatus
	}{
		{"pago→rascunho", OrderStatusPago, OrderStatusRascunho},
		{"cancelado→confirmado", OrderStatusCancelado, OrderStatusConfirmado},
		{"cancelado→pago", OrderStatusCancelado, OrderStatusPago},
		{"cancelado→rascunho", OrderStatusCancelado, OrderStatusRascunho},
		{"pago→confirmado", OrderStatusPago, OrderStatusConfirmado},
		{"pago→pago (auto-transição)", OrderStatusPago, OrderStatusPago},
		{"rascunho→pago (pula estado)", OrderStatusRascunho, OrderStatusPago},
		{"rascunho→cancelado (pula estado)", OrderStatusRascunho, OrderStatusCancelado},
		{"confirmado→rascunho (retrocesso)", OrderStatusConfirmado, OrderStatusRascunho},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.from.Transition(tt.to)
			require.Error(t, err, "transição inválida deve retornar erro")
			assert.True(t, errors.Is(err, ErrInvalidTransition),
				"erro deve ser ErrInvalidTransition, obteve: %v", err)
		})
	}
}

// TestOrderTransition_PagoCancelado_RequiresReversalCheck verifica a regra de negócio
// específica: pago→cancelado sinaliza RequiresReversalCheck=true (FR-028).
func TestOrderTransition_PagoCancelado_RequiresReversalCheck(t *testing.T) {
	result, err := OrderStatusPago.Transition(OrderStatusCancelado)
	require.NoError(t, err)
	assert.True(t, result.RequiresReversalCheck,
		"pago→cancelado deve sinalizar RequiresReversalCheck=true")
	assert.Equal(t, OrderStatusPago, result.From)
	assert.Equal(t, OrderStatusCancelado, result.To)
}

// TestOrderTransition_OutrasCancelacoes_SemReversalCheck garante que apenas
// pago→cancelado exige reversão — confirmado→cancelado não.
func TestOrderTransition_OutrasCancelacoes_SemReversalCheck(t *testing.T) {
	result, err := OrderStatusConfirmado.Transition(OrderStatusCancelado)
	require.NoError(t, err)
	assert.False(t, result.RequiresReversalCheck,
		"confirmado→cancelado NÃO deve exigir reversão (comissão ainda não calculada)")
}

// TestOrderTransition_ErroContemMensagemInformativa verifica que o erro
// contém informação sobre a transição inválida tentada.
func TestOrderTransition_ErroContemMensagemInformativa(t *testing.T) {
	_, err := OrderStatusPago.Transition(OrderStatusRascunho)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pago")
	assert.Contains(t, err.Error(), "rascunho")
}
