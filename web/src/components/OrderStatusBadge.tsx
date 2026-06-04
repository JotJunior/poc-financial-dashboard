// OrderStatusBadge — badge visual + botões de transição válidos (Task 8.3.3)
// Máquina de estado: rascunho→confirmado, confirmado→pago, confirmado→cancelado, pago→cancelado
import type { OrderStatus } from '../types/order';

const STATUS_LABELS: Record<OrderStatus, string> = {
  rascunho: 'Rascunho',
  confirmado: 'Confirmado',
  pago: 'Pago',
  cancelado: 'Cancelado',
};

const STATUS_COLORS: Record<OrderStatus, { bg: string; color: string }> = {
  rascunho: { bg: '#2b2b40', color: '#89b4fa' },
  confirmado: { bg: '#1a2b3b', color: '#89dceb' },
  pago: { bg: '#1a3b2b', color: '#a6e3a1' },
  cancelado: { bg: '#3b1a1a', color: '#f38ba8' },
};

// Transições válidas conforme spec FR-008
const VALID_TRANSITIONS: Record<OrderStatus, OrderStatus[]> = {
  rascunho: ['confirmado'],
  confirmado: ['pago', 'cancelado'],
  pago: ['cancelado'],
  cancelado: [],
};

interface OrderStatusBadgeProps {
  status: OrderStatus;
  onTransition?: (to: OrderStatus) => void | Promise<void>;
  isLoading?: boolean;
  /** Se false, ocultar botões de transição (read-only) */
  interactive?: boolean;
}

export function OrderStatusBadge({
  status,
  onTransition,
  isLoading,
  interactive = true,
}: OrderStatusBadgeProps) {
  const { bg, color } = STATUS_COLORS[status];
  const transitions = VALID_TRANSITIONS[status];

  const TRANS_LABELS: Partial<Record<OrderStatus, string>> = {
    confirmado: 'Confirmar',
    pago: 'Marcar Pago',
    cancelado: 'Cancelar',
  };

  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', flexWrap: 'wrap' }}>
      <span style={{
        padding: '3px 10px',
        borderRadius: '9999px',
        fontSize: '0.75rem',
        fontWeight: 600,
        background: bg,
        color,
      }}>
        {STATUS_LABELS[status]}
      </span>

      {interactive && transitions.length > 0 && onTransition && (
        <div style={{ display: 'flex', gap: '0.4rem' }}>
          {transitions.map(to => (
            <button
              key={to}
              onClick={() => onTransition(to)}
              disabled={isLoading}
              style={{
                padding: '3px 10px',
                borderRadius: '6px',
                border: `1px solid ${STATUS_COLORS[to].color}`,
                background: 'transparent',
                color: STATUS_COLORS[to].color,
                cursor: isLoading ? 'not-allowed' : 'pointer',
                fontSize: '0.75rem',
                opacity: isLoading ? 0.6 : 1,
              }}
            >
              {TRANS_LABELS[to] ?? STATUS_LABELS[to]}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
