// Commissions — lista com filtros, apuração, transições de estado (Task 8.4.1)
// Exibe saldo líquido (net) além do bruto (Task 8.4.4)
import { useState } from 'react';
import { useCommissions, useTransitionCommission } from '../api/commissions';
import { useVendors } from '../api/vendors';
import { CommissionStatusBadge } from '../components/CommissionStatusBadge';
import { ApurationForm } from '../components/ApurationForm';
import { useAuth } from '../api/auth-context';
import { centsToDisplay } from '../components/OrderForm';
import type { CommissionStatus } from '../types/commission';

export function Commissions() {
  const { user } = useAuth();
  const isFinanceiro = user?.role === 'financeiro';
  const isGestor = user?.role === 'gestor';
  const canApurate = isGestor;

  const [filterStatus, setFilterStatus] = useState<CommissionStatus | ''>('');
  const [filterVendor, setFilterVendor] = useState('');
  const [filterYear, setFilterYear] = useState('');
  const [filterMonth, setFilterMonth] = useState('');

  const { data: commissions = [], isLoading, isError } = useCommissions({
    status: (filterStatus || undefined) as CommissionStatus | undefined,
    vendorId: filterVendor || undefined,
    year: filterYear ? parseInt(filterYear, 10) : undefined,
    month: filterMonth ? parseInt(filterMonth, 10) : undefined,
  });
  // Vendedor não tem permissão em GET /vendors (403) e nem precisa do mapa —
  // só vê as próprias comissões. Disparar a query apenas para Gestor/Financeiro.
  const { data: vendors = [] } = useVendors(isGestor || isFinanceiro);
  const transitionComm = useTransitionCommission();

  const vendorName = (id: string) => vendors.find(v => v.id === id)?.name ?? id.slice(0, 8) + '…';

  const selectStyle = {
    padding: '6px 10px', borderRadius: '6px', border: '1px solid #45475a',
    background: '#313244', color: '#cdd6f4', fontSize: '0.85rem',
  };

  if (isError) {
    return <div style={{ color: '#f38ba8', padding: '1rem' }}>Erro ao carregar comissões.</div>;
  }

  return (
    <div style={{ maxWidth: '950px', margin: '0 auto' }}>
      <h1 style={{ fontSize: '1.4rem', color: '#89dceb', margin: '0 0 1.25rem' }}>Comissões</h1>

      {/* Apuração — apenas Gestor */}
      {canApurate && (
        <div style={{ marginBottom: '1.5rem' }}>
          <ApurationForm />
        </div>
      )}

      {/* Filtros */}
      <div style={{ display: 'flex', gap: '0.75rem', flexWrap: 'wrap', marginBottom: '1.25rem' }}>
        <select value={filterStatus} onChange={e => setFilterStatus(e.target.value as CommissionStatus | '')} style={selectStyle}>
          <option value="">Todos os status</option>
          <option value="pendente">Pendente</option>
          <option value="aprovado">Aprovado</option>
          <option value="pago">Pago</option>
        </select>

        {(isGestor || isFinanceiro) && (
          <select value={filterVendor} onChange={e => setFilterVendor(e.target.value)} style={selectStyle}>
            <option value="">Todos os vendedores</option>
            {vendors.map(v => <option key={v.id} value={v.id}>{v.name}</option>)}
          </select>
        )}

        <select value={filterYear} onChange={e => setFilterYear(e.target.value)} style={selectStyle}>
          <option value="">Ano</option>
          {[2024, 2025, 2026].map(y => <option key={y} value={y}>{y}</option>)}
        </select>

        <select value={filterMonth} onChange={e => setFilterMonth(e.target.value)} style={selectStyle}>
          <option value="">Mês</option>
          {Array.from({ length: 12 }, (_, i) => (
            <option key={i + 1} value={i + 1}>
              {new Date(2000, i).toLocaleString('pt-BR', { month: 'long' })}
            </option>
          ))}
        </select>

        <span style={{ alignSelf: 'center', color: '#6c7086', fontSize: '0.85rem', marginLeft: 'auto' }}>
          {commissions.length} comissõe{commissions.length !== 1 ? 's' : ''}
        </span>
      </div>

      {/* Lista */}
      {isLoading ? (
        <div style={{ color: '#a6adc8', textAlign: 'center', padding: '2rem' }}>Carregando…</div>
      ) : commissions.length === 0 ? (
        <div style={{ color: '#6c7086', textAlign: 'center', padding: '2rem' }}>
          Nenhuma comissão encontrada.
        </div>
      ) : (
        commissions.map(comm => {
          const hasReversal = comm.netCents < comm.valueCents;
          return (
            <div key={comm.id} style={{
              background: '#181825', borderRadius: '10px',
              padding: '1rem 1.25rem', border: '1px solid #2e2e4a', marginBottom: '0.75rem',
            }}>
              <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '1rem' }}>
                <div style={{ flex: 1 }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', marginBottom: '4px' }}>
                    <code style={{ color: '#cba6f7', fontSize: '0.78rem' }}>{comm.id.slice(0, 8)}…</code>
                    <span style={{ color: '#6c7086', fontSize: '0.8rem' }}>
                      {comm.periodYear}/{String(comm.periodMonth).padStart(2, '0')}
                    </span>
                  </div>
                  <div style={{ color: '#a6adc8', fontSize: '0.85rem', marginBottom: '6px' }}>
                    {vendorName(comm.vendorId)}
                  </div>

                  {/* Valores bruto + líquido */}
                  <div style={{ display: 'flex', gap: '1.5rem', flexWrap: 'wrap' }}>
                    <div>
                      <span style={{ color: '#6c7086', fontSize: '0.75rem' }}>Bruto: </span>
                      <span style={{ color: '#cdd6f4', fontWeight: 600 }}>{centsToDisplay(comm.valueCents)}</span>
                    </div>
                    {hasReversal && (
                      <div>
                        <span style={{ color: '#6c7086', fontSize: '0.75rem' }}>Líquido: </span>
                        <span style={{ color: '#fab387', fontWeight: 600 }}>{centsToDisplay(comm.netCents)}</span>
                        <span style={{ color: '#f38ba8', fontSize: '0.75rem', marginLeft: '4px' }}>
                          (estorno: {centsToDisplay(comm.netCents - comm.valueCents)})
                        </span>
                      </div>
                    )}
                    <div>
                      <span style={{ color: '#6c7086', fontSize: '0.75rem' }}>Taxa: </span>
                      <span style={{ color: '#cdd6f4' }}>
                        {parseFloat(comm.appliedPercentage).toLocaleString('pt-BR', { minimumFractionDigits: 2 })}%
                      </span>
                    </div>
                  </div>
                </div>

                <CommissionStatusBadge
                  status={comm.status}
                  // Transição de status de comissão (aprovar/pagar/reverter) é
                  // exclusiva do Financeiro (spec US4; backend requireRole("financeiro")).
                  // O Gestor apenas apura — habilitar o controle para ele gerava 403.
                  interactive={isFinanceiro}
                  isLoading={transitionComm.isPending}
                  onTransition={(to, motivo) => {
                    void transitionComm.mutateAsync({ id: comm.id, status: to, motivo });
                  }}
                />
              </div>
            </div>
          );
        })
      )}
    </div>
  );
}
