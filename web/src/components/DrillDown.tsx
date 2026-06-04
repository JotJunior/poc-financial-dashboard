// DrillDown — rastreabilidade pedido → comissão → estorno (Task 8.5.4)
// SC-004: <= 3 cliques para rastreabilidade
import { useState } from 'react';
import { useDrillDown } from '../api/dashboard';
import { centsToDisplay } from './OrderForm';

interface DrillDownProps {
  orderId: string;
  onClose: () => void;
}

export function DrillDown({ orderId, onClose }: DrillDownProps) {
  const { data, isLoading, isError } = useDrillDown(orderId);

  return (
    <div style={{
      position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.7)',
      display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1000,
    }}>
      <div style={{
        background: '#181825', borderRadius: '16px', padding: '1.5rem',
        width: '100%', maxWidth: '600px', maxHeight: '80vh', overflowY: 'auto',
        border: '1px solid #2e2e4a',
      }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1.25rem' }}>
          <h2 style={{ color: '#cba6f7', margin: 0, fontSize: '1.1rem' }}>
            Rastreabilidade do Pedido
          </h2>
          <button onClick={onClose} style={{
            background: 'transparent', border: 'none', color: '#a6adc8',
            cursor: 'pointer', fontSize: '1.2rem',
          }}>
            ×
          </button>
        </div>

        {isLoading && <p style={{ color: '#a6adc8' }}>Carregando…</p>}
        {isError && <p style={{ color: '#f38ba8' }}>Erro ao carregar detalhes.</p>}

        {data && (
          <div>
            {/* Pedido */}
            <div style={{
              background: '#1e1e2e', borderRadius: '8px', padding: '1rem',
              border: '1px solid #45475a', marginBottom: '1rem',
            }}>
              <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                <div>
                  <p style={{ color: '#6c7086', fontSize: '0.75rem', marginBottom: '2px' }}>Pedido</p>
                  <code style={{ color: '#cba6f7', fontSize: '0.85rem' }}>{data.orderId.slice(0, 8)}…</code>
                </div>
                <div>
                  <p style={{ color: '#6c7086', fontSize: '0.75rem', marginBottom: '2px' }}>Data</p>
                  <p style={{ color: '#cdd6f4', fontSize: '0.85rem' }}>
                    {new Date(data.orderDate).toLocaleDateString('pt-BR')}
                  </p>
                </div>
                <div>
                  <p style={{ color: '#6c7086', fontSize: '0.75rem', marginBottom: '2px' }}>Total</p>
                  <p style={{ color: '#a6e3a1', fontWeight: 700 }}>{centsToDisplay(data.totalCents)}</p>
                </div>
                <div>
                  <p style={{ color: '#6c7086', fontSize: '0.75rem', marginBottom: '2px' }}>Status</p>
                  <p style={{ color: '#cdd6f4', fontSize: '0.85rem' }}>{data.status}</p>
                </div>
              </div>
            </div>

            {/* Comissões */}
            {data.commissions.length === 0 ? (
              <p style={{ color: '#6c7086', fontSize: '0.85rem' }}>Nenhuma comissão calculada.</p>
            ) : (
              data.commissions.map(comm => (
                <div key={comm.commissionId} style={{
                  background: '#1e1e2e', borderRadius: '8px', padding: '1rem',
                  border: '1px solid #2e2e4a', marginBottom: '0.75rem',
                }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '0.5rem' }}>
                    <div>
                      <p style={{ color: '#6c7086', fontSize: '0.72rem', marginBottom: '2px' }}>Comissão</p>
                      <code style={{ color: '#89b4fa', fontSize: '0.82rem' }}>
                        {comm.commissionId.slice(0, 8)}…
                      </code>
                    </div>
                    <div>
                      <p style={{ color: '#6c7086', fontSize: '0.72rem', marginBottom: '2px' }}>Período</p>
                      <p style={{ color: '#cdd6f4', fontSize: '0.82rem' }}>
                        {comm.periodYear}/{String(comm.periodMonth).padStart(2, '0')}
                      </p>
                    </div>
                    <div>
                      <p style={{ color: '#6c7086', fontSize: '0.72rem', marginBottom: '2px' }}>Bruto</p>
                      <p style={{ color: '#a6e3a1', fontWeight: 600 }}>{centsToDisplay(comm.valueCents)}</p>
                    </div>
                    <div>
                      <p style={{ color: '#6c7086', fontSize: '0.72rem', marginBottom: '2px' }}>Líquido</p>
                      <p style={{ color: comm.netCents < comm.valueCents ? '#fab387' : '#a6e3a1', fontWeight: 600 }}>
                        {centsToDisplay(comm.netCents)}
                      </p>
                    </div>
                    <div>
                      <p style={{ color: '#6c7086', fontSize: '0.72rem', marginBottom: '2px' }}>Status</p>
                      <p style={{ color: '#cdd6f4', fontSize: '0.82rem' }}>{comm.status}</p>
                    </div>
                  </div>

                  {/* Estornos */}
                  {comm.reversals.length > 0 && (
                    <div style={{ borderTop: '1px solid #2e2e4a', paddingTop: '0.75rem', marginTop: '0.5rem' }}>
                      <p style={{ color: '#fab387', fontSize: '0.78rem', marginBottom: '0.5rem' }}>
                        Estornos ({comm.reversals.length})
                      </p>
                      {comm.reversals.map(rev => (
                        <div key={rev.reversalId} style={{
                          display: 'flex', gap: '1rem', fontSize: '0.78rem', color: '#a6adc8',
                          padding: '4px 0',
                        }}>
                          <code style={{ color: '#f38ba8' }}>{rev.reversalId.slice(0, 8)}…</code>
                          <span style={{ color: '#f38ba8', fontWeight: 600 }}>{centsToDisplay(rev.valueCents)}</span>
                          <span>{rev.status}</span>
                          <span>{new Date(rev.createdAt).toLocaleDateString('pt-BR')}</span>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              ))
            )}
          </div>
        )}
      </div>
    </div>
  );
}

// Wrapper com estado de visibilidade — SC-004: <= 2 cliques para abrir drill-down
interface DrillDownTriggerProps {
  orderId: string;
  children: React.ReactNode;
}

export function DrillDownTrigger({ orderId, children }: DrillDownTriggerProps) {
  const [open, setOpen] = useState(false);

  return (
    <>
      <button
        onClick={() => setOpen(true)}
        style={{
          background: 'transparent', border: 'none', cursor: 'pointer',
          color: 'inherit', padding: 0, textDecoration: 'underline dotted',
        }}
      >
        {children}
      </button>
      {open && <DrillDown orderId={orderId} onClose={() => setOpen(false)} />}
    </>
  );
}
