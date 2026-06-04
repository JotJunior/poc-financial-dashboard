package domain

import (
	"errors"
	"fmt"
)

// OrderStatus representa o estado de um pedido.
// Constitution P-I: toda transição é auditável — o caller DEVE registrar
// em audit_trail após cada Transition bem-sucedida.
type OrderStatus string

const (
	OrderStatusRascunho  OrderStatus = "rascunho"
	OrderStatusConfirmado OrderStatus = "confirmado"
	OrderStatusPago      OrderStatus = "pago"
	OrderStatusCancelado OrderStatus = "cancelado"
)

// ErrInvalidTransition é retornado quando a transição solicitada não é
// permitida pela máquina de estados (FR-008).
var ErrInvalidTransition = errors.New("transição de estado inválida")

// TransitionResult carrega o resultado de uma transição bem-sucedida.
// RequiresReversalCheck sinaliza que o service DEVE verificar/criar estorno
// (caso pago→cancelado — FR-027/FR-028).
type TransitionResult struct {
	From                 OrderStatus
	To                   OrderStatus
	RequiresReversalCheck bool
}

// Transition valida e executa a transição de estado de s para to.
// Retorna TransitionResult com metadados da transição, ou ErrInvalidTransition.
//
// Transições válidas (FR-008):
//   - rascunho   → confirmado
//   - confirmado → pago
//   - confirmado → cancelado
//   - pago       → cancelado  (RequiresReversalCheck=true)
//
// Todas as demais combinações retornam ErrInvalidTransition.
func (s OrderStatus) Transition(to OrderStatus) (TransitionResult, error) {
	type edge struct{ from, to OrderStatus }

	valid := map[edge]bool{
		{OrderStatusRascunho, OrderStatusConfirmado}:  true,
		{OrderStatusConfirmado, OrderStatusPago}:      true,
		{OrderStatusConfirmado, OrderStatusCancelado}: true,
		{OrderStatusPago, OrderStatusCancelado}:       true,
	}

	e := edge{s, to}
	if !valid[e] {
		return TransitionResult{}, fmt.Errorf("%w: %s → %s", ErrInvalidTransition, s, to)
	}

	result := TransitionResult{
		From: s,
		To:   to,
		// pago→cancelado exige verificação/criação de estorno (FR-028)
		RequiresReversalCheck: s == OrderStatusPago && to == OrderStatusCancelado,
	}

	return result, nil
}
