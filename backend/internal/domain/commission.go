package domain

import (
	"errors"
	"fmt"
)

// CommissionStatus representa o estado de uma comissão.
// Constitution P-I: toda transição é auditável — o caller DEVE registrar
// em audit_trail após cada Transition bem-sucedida.
type CommissionStatus string

const (
	CommissionStatusPendente  CommissionStatus = "pendente"
	CommissionStatusAprovado  CommissionStatus = "aprovado"
	CommissionStatusPago      CommissionStatus = "pago"
)

// ReversalStatus representa o estado de um estorno de comissão (FR-027/FR-028).
// A bifurcação entre aplicado e pendente_aprovacao depende do estado da comissão
// no momento do cancelamento do pedido.
type ReversalStatus string

const (
	ReversalStatusAplicado           ReversalStatus = "aplicado"
	ReversalStatusPendenteAprovacao  ReversalStatus = "pendente_aprovacao"
	ReversalStatusAprovado           ReversalStatus = "aprovado"
	ReversalStatusLancado            ReversalStatus = "lancado"
)

// ErrMissingReason é retornado quando a transição aprovado→pendente é tentada
// sem fornecer motivo obrigatório.
var ErrMissingReason = errors.New("motivo obrigatório para esta transição")

// CommissionTransitionResult carrega o resultado de uma transição bem-sucedida.
type CommissionTransitionResult struct {
	From   CommissionStatus
	To     CommissionStatus
	Reason string // preenchido apenas quando aprovado→pendente
}

// Transition valida e executa a transição de estado de s para to.
// Para a transição aprovado→pendente, reason é obrigatório (retorna ErrMissingReason se vazio).
//
// Transições válidas (FR-020):
//   - pendente  → aprovado
//   - aprovado  → pago
//   - aprovado  → pendente  (requer reason não-vazio)
//
// Todas as demais combinações retornam ErrInvalidTransition.
func (s CommissionStatus) Transition(to CommissionStatus, reason string) (CommissionTransitionResult, error) {
	type edge struct{ from, to CommissionStatus }

	valid := map[edge]bool{
		{CommissionStatusPendente, CommissionStatusAprovado}: true,
		{CommissionStatusAprovado, CommissionStatusPago}:    true,
		{CommissionStatusAprovado, CommissionStatusPendente}: true,
	}

	e := edge{s, to}
	if !valid[e] {
		return CommissionTransitionResult{}, fmt.Errorf("%w: %s → %s", ErrInvalidTransition, s, to)
	}

	// aprovado→pendente exige motivo explícito
	if s == CommissionStatusAprovado && to == CommissionStatusPendente {
		if reason == "" {
			return CommissionTransitionResult{}, fmt.Errorf("%w: aprovado → pendente requer motivo", ErrMissingReason)
		}
	}

	return CommissionTransitionResult{
		From:   s,
		To:     to,
		Reason: reason,
	}, nil
}

// DetermineReversalStatus implementa a lógica de bifurcação do estorno (FR-028):
//   - comissão pendente  → estorno aplicado     (sem aprovação necessária)
//   - comissão aprovado  → estorno pendente_aprovacao
//   - comissão pago      → estorno pendente_aprovacao
//
// Essa função é pura e determinística — usada pelo service ao criar um estorno
// decorrente de cancelamento de pedido.
func DetermineReversalStatus(cs CommissionStatus) ReversalStatus {
	switch cs {
	case CommissionStatusPendente:
		return ReversalStatusAplicado
	case CommissionStatusAprovado, CommissionStatusPago:
		return ReversalStatusPendenteAprovacao
	default:
		// estado desconhecido — conservativo: exige aprovação
		return ReversalStatusPendenteAprovacao
	}
}
