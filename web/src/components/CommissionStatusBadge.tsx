// CommissionStatusBadge — badge com transições para Financeiro (Task 8.4.3)
// Transições válidas: pendente→aprovado, aprovado→pago, aprovado→pendente (com motivo)
import { useState } from 'react';
import type { CommissionStatus } from '../types/commission';

const STATUS_LABELS: Record<CommissionStatus, string> = {
  pendente: 'Pendente',
  aprovado: 'Aprovado',
  pago: 'Pago',
};

const STATUS_COLORS: Record<CommissionStatus, { bg: string; color: string }> = {
  pendente: { bg: '#2b2520', color: '#fab387' },
  aprovado: { bg: '#1a2b3b', color: '#89dceb' },
  pago: { bg: '#1a3b2b', color: '#a6e3a1' },
};

const VALID_TRANSITIONS: Record<CommissionStatus, CommissionStatus[]> = {
  pendente: ['aprovado'],
  aprovado: ['pago', 'pendente'],
  pago: [],
};

const TRANS_LABELS: Partial<Record<CommissionStatus, string>> = {
  aprovado: 'Aprovar',
  pago: 'Marcar Pago',
  pendente: 'Reverter',
};

interface CommissionStatusBadgeProps {
  status: CommissionStatus;
  onTransition?: (to: CommissionStatus, motivo?: string) => void | Promise<void>;
  isLoading?: boolean;
  interactive?: boolean;
}

export function CommissionStatusBadge({
  status,
  onTransition,
  isLoading,
  interactive = true,
}: CommissionStatusBadgeProps) {
  const { bg, color } = STATUS_COLORS[status];
  const transitions = VALID_TRANSITIONS[status];
  const [motivoModal, setMotivoModal] = useState<CommissionStatus | null>(null);
  const [motivo, setMotivo] = useState('');

  async function handleTransition(to: CommissionStatus) {
    // Reverter aprovado→pendente requer motivo
    if (to === 'pendente' && status === 'aprovado') {
      setMotivoModal(to);
      return;
    }
    await onTransition?.(to);
  }

  async function confirmWithMotivo() {
    if (!motivo.trim()) return;
    await onTransition?.(motivoModal!, motivo);
    setMotivoModal(null);
    setMotivo('');
  }

  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', flexWrap: 'wrap' }}>
        <span style={{
          padding: '3px 10px', borderRadius: '9999px', fontSize: '0.75rem', fontWeight: 600,
          background: bg, color,
        }}>
          {STATUS_LABELS[status]}
        </span>

        {interactive && transitions.length > 0 && onTransition && (
          <div style={{ display: 'flex', gap: '0.4rem' }}>
            {transitions.map(to => (
              <button
                key={to}
                onClick={() => handleTransition(to)}
                disabled={isLoading}
                style={{
                  padding: '3px 10px', borderRadius: '6px',
                  border: `1px solid ${STATUS_COLORS[to].color}`,
                  background: 'transparent', color: STATUS_COLORS[to].color,
                  cursor: isLoading ? 'not-allowed' : 'pointer',
                  fontSize: '0.75rem', opacity: isLoading ? 0.6 : 1,
                }}
              >
                {TRANS_LABELS[to] ?? STATUS_LABELS[to]}
              </button>
            ))}
          </div>
        )}
      </div>

      {/* Modal de motivo para reversão */}
      {motivoModal && (
        <div style={{
          position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.7)',
          display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1000,
        }}>
          <div style={{
            background: '#181825', borderRadius: '12px', padding: '1.5rem',
            width: '400px', border: '1px solid #45475a',
          }}>
            <h3 style={{ color: '#cdd6f4', marginBottom: '0.75rem' }}>Motivo da Reversão</h3>
            <textarea
              value={motivo}
              onChange={e => setMotivo(e.target.value)}
              placeholder="Descreva o motivo para reverter a aprovação…"
              style={{
                width: '100%', padding: '8px 12px', borderRadius: '6px',
                border: '1px solid #45475a', background: '#313244', color: '#cdd6f4',
                fontSize: '0.9rem', minHeight: '80px', resize: 'vertical',
                boxSizing: 'border-box',
              }}
            />
            <div style={{ display: 'flex', gap: '0.75rem', justifyContent: 'flex-end', marginTop: '1rem' }}>
              <button onClick={() => { setMotivoModal(null); setMotivo(''); }} style={{
                padding: '7px 14px', borderRadius: '6px', border: '1px solid #45475a',
                background: 'transparent', color: '#a6adc8', cursor: 'pointer',
              }}>
                Cancelar
              </button>
              <button
                onClick={confirmWithMotivo}
                disabled={!motivo.trim() || isLoading}
                style={{
                  padding: '7px 14px', borderRadius: '6px', border: 'none',
                  background: '#fab387', color: '#1e1e2e', fontWeight: 600,
                  cursor: !motivo.trim() || isLoading ? 'not-allowed' : 'pointer',
                  opacity: !motivo.trim() || isLoading ? 0.7 : 1,
                }}
              >
                Confirmar Reversão
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
