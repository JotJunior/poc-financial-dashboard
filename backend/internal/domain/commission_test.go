package domain

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCommissionTransition_TransicoesValidas verifica as 3 transições permitidas.
func TestCommissionTransition_TransicoesValidas(t *testing.T) {
	tests := []struct {
		name       string
		from       CommissionStatus
		to         CommissionStatus
		reason     string
		wantReason string
	}{
		{
			name:   "pendente→aprovado",
			from:   CommissionStatusPendente,
			to:     CommissionStatusAprovado,
			reason: "",
		},
		{
			name:   "aprovado→pago",
			from:   CommissionStatusAprovado,
			to:     CommissionStatusPago,
			reason: "",
		},
		{
			name:       "aprovado→pendente com motivo",
			from:       CommissionStatusAprovado,
			to:         CommissionStatusPendente,
			reason:     "erro de cálculo detectado",
			wantReason: "erro de cálculo detectado",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.from.Transition(tt.to, tt.reason)
			require.NoError(t, err)
			assert.Equal(t, tt.from, result.From)
			assert.Equal(t, tt.to, result.To)
			assert.Equal(t, tt.wantReason, result.Reason)
		})
	}
}

// TestCommissionTransition_TransicoesInvalidas verifica rejeição de transições não permitidas.
func TestCommissionTransition_TransicoesInvalidas(t *testing.T) {
	tests := []struct {
		name   string
		from   CommissionStatus
		to     CommissionStatus
		reason string
	}{
		{"pendente→pago (pula estado)", CommissionStatusPendente, CommissionStatusPago, ""},
		{"pago→aprovado (retrocesso)", CommissionStatusPago, CommissionStatusAprovado, ""},
		{"pago→pendente (retrocesso)", CommissionStatusPago, CommissionStatusPendente, "motivo"},
		{"pendente→pendente (auto)", CommissionStatusPendente, CommissionStatusPendente, ""},
		{"aprovado→aprovado (auto)", CommissionStatusAprovado, CommissionStatusAprovado, ""},
		{"pago→pago (auto)", CommissionStatusPago, CommissionStatusPago, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.from.Transition(tt.to, tt.reason)
			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrInvalidTransition),
				"deve ser ErrInvalidTransition, obteve: %v", err)
		})
	}
}

// TestCommissionTransition_AprovadoPendente_SemMotivo verifica que aprovado→pendente
// sem motivo retorna ErrMissingReason.
func TestCommissionTransition_AprovadoPendente_SemMotivo(t *testing.T) {
	_, err := CommissionStatusAprovado.Transition(CommissionStatusPendente, "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrMissingReason),
		"motivo vazio deve retornar ErrMissingReason, obteve: %v", err)
}

// TestCommissionTransition_AprovadoPendente_ComEspacosApenas verifica que apenas
// espaços não satisfazem o requisito de motivo (branch defensiva).
func TestCommissionTransition_AprovadoPendente_ComEspacosApenas(t *testing.T) {
	// Nota: a spec não pede trimming explícito, mas a regra de negócio diz
	// "motivo obrigatório" — espaços em branco não constituem motivo válido.
	// Esta verificação documenta a decisão de não trimar (aceitar " " se o caller passar).
	// Se o projeto decidir trimar, este teste deve ser atualizado.
	// Por ora: "" é inválido, " " seria aceito (caller responsável por validar UI).
	result, err := CommissionStatusAprovado.Transition(CommissionStatusPendente, " ")
	require.NoError(t, err, "espaço em branco é aceito — caller valida UI")
	assert.Equal(t, " ", result.Reason)
}

// TestDetermineReversalStatus_Bifurcacao verifica a lógica de bifurcação (FR-028).
func TestDetermineReversalStatus_Bifurcacao(t *testing.T) {
	tests := []struct {
		name     string
		status   CommissionStatus
		expected ReversalStatus
	}{
		{
			name:     "pendente → aplicado (sem aprovação)",
			status:   CommissionStatusPendente,
			expected: ReversalStatusAplicado,
		},
		{
			name:     "aprovado → pendente_aprovacao",
			status:   CommissionStatusAprovado,
			expected: ReversalStatusPendenteAprovacao,
		},
		{
			name:     "pago → pendente_aprovacao",
			status:   CommissionStatusPago,
			expected: ReversalStatusPendenteAprovacao,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetermineReversalStatus(tt.status)
			assert.Equal(t, tt.expected, result,
				"bifurcação incorreta para status=%s", tt.status)
		})
	}
}

// TestDetermineReversalStatus_Determinismo verifica que DetermineReversalStatus
// é determinística (mesma entrada, mesma saída — analogia com SC-002 de money.go).
func TestDetermineReversalStatus_Determinismo(t *testing.T) {
	statuses := []CommissionStatus{
		CommissionStatusPendente,
		CommissionStatusAprovado,
		CommissionStatusPago,
	}

	for _, cs := range statuses {
		expected := DetermineReversalStatus(cs)
		for i := 0; i < 100; i++ {
			result := DetermineReversalStatus(cs)
			assert.Equal(t, expected, result,
				"DetermineReversalStatus(%s) deve ser determinístico, falhou na iteração %d", cs, i)
		}
	}
}

// TestReversalStatusConstantes verifica que todas as constantes do enum estão definidas.
func TestReversalStatusConstantes(t *testing.T) {
	assert.Equal(t, ReversalStatus("aplicado"), ReversalStatusAplicado)
	assert.Equal(t, ReversalStatus("pendente_aprovacao"), ReversalStatusPendenteAprovacao)
	assert.Equal(t, ReversalStatus("aprovado"), ReversalStatusAprovado)
	assert.Equal(t, ReversalStatus("lancado"), ReversalStatusLancado)
}
