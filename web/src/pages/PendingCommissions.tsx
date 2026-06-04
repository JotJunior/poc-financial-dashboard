// PendingCommissions — indicadores de comissões pendentes (Task 8.5.5, FR-019)
// Apenas Gestor e Financeiro
import { usePendingCommissions } from '../api/dashboard';
import { centsToDisplay } from '../components/OrderForm';
import { Link } from 'react-router-dom';

export function PendingCommissions() {
  const { data, isLoading, isError } = usePendingCommissions();

  if (isError) {
    return (
      <div style={{ color: '#f38ba8', padding: '1rem' }}>
        Sem permissão ou erro ao carregar pendências.
      </div>
    );
  }

  return (
    <div style={{ maxWidth: '700px', margin: '0 auto' }}>
      <h1 style={{ fontSize: '1.4rem', color: '#fab387', margin: '0 0 1.25rem' }}>
        Comissões Pendentes
      </h1>

      {isLoading ? (
        <div style={{ color: '#a6adc8', textAlign: 'center', padding: '2rem' }}>Carregando…</div>
      ) : data ? (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
          {/* Pendentes de aprovação */}
          <div style={{
            background: '#181825', borderRadius: '12px', padding: '1.5rem',
            border: '1px solid #fab387',
          }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <div>
                <p style={{ color: '#6c7086', fontSize: '0.8rem', marginBottom: '4px' }}>
                  Pendentes de Aprovação
                </p>
                <p style={{ color: '#fab387', fontSize: '1.6rem', fontWeight: 700, margin: 0 }}>
                  {centsToDisplay(data.pendingApprovalCents)}
                </p>
                <p style={{ color: '#6c7086', fontSize: '0.78rem', marginTop: '3px' }}>
                  {data.pendingApprovalCount} comissõe{data.pendingApprovalCount !== 1 ? 's' : ''}
                </p>
              </div>
              <Link to="/commissions?status=pendente" style={{
                padding: '8px 16px', borderRadius: '8px', border: '1px solid #fab387',
                color: '#fab387', textDecoration: 'none', fontSize: '0.85rem',
              }}>
                Ver Pendentes
              </Link>
            </div>
          </div>

          {/* Aprovadas a pagar */}
          <div style={{
            background: '#181825', borderRadius: '12px', padding: '1.5rem',
            border: '1px solid #89dceb',
          }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <div>
                <p style={{ color: '#6c7086', fontSize: '0.8rem', marginBottom: '4px' }}>
                  Aprovadas — Aguardando Pagamento
                </p>
                <p style={{ color: '#89dceb', fontSize: '1.6rem', fontWeight: 700, margin: 0 }}>
                  {centsToDisplay(data.approvedUnpaidCents)}
                </p>
                <p style={{ color: '#6c7086', fontSize: '0.78rem', marginTop: '3px' }}>
                  {data.approvedUnpaidCount} comissõe{data.approvedUnpaidCount !== 1 ? 's' : ''}
                </p>
              </div>
              <Link to="/commissions?status=aprovado" style={{
                padding: '8px 16px', borderRadius: '8px', border: '1px solid #89dceb',
                color: '#89dceb', textDecoration: 'none', fontSize: '0.85rem',
              }}>
                Ver Aprovadas
              </Link>
            </div>
          </div>

          {/* Alerta se há pendências */}
          {(data.pendingApprovalCount > 0 || data.approvedUnpaidCount > 0) && (
            <div style={{
              background: '#2b1a00', borderRadius: '8px', padding: '1rem',
              border: '1px solid #fab387',
            }}>
              <p style={{ color: '#fab387', fontSize: '0.85rem', margin: 0 }}>
                Existem comissões aguardando ação. Acesse a página de{' '}
                <Link to="/commissions" style={{ color: '#fab387' }}>Comissões</Link>{' '}
                para processar.
              </p>
            </div>
          )}
        </div>
      ) : null}
    </div>
  );
}
