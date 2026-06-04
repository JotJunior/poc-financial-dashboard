// DashboardConsolidated — Gestor/Financeiro: métricas agregadas (Task 8.5.1)
// Filtro por período, ranking de vendedores, gráficos recharts
import { useState } from 'react';
import { useConsolidatedDashboard } from '../api/dashboard';
import { VendorBarChart } from '../components/SalesChart';
import { centsToDisplay } from '../components/OrderForm';
import type { DashboardFilter } from '../types/dashboard';

function MetricCard({ label, value, sub, color = '#cdd6f4' }: {
  label: string; value: string; sub?: string; color?: string;
}) {
  return (
    <div style={{
      background: '#181825', borderRadius: '12px', padding: '1.25rem',
      border: '1px solid #2e2e4a', flex: 1, minWidth: '160px',
    }}>
      <p style={{ color: '#6c7086', fontSize: '0.75rem', marginBottom: '4px' }}>{label}</p>
      <p style={{ color, fontSize: '1.4rem', fontWeight: 700, margin: 0 }}>{value}</p>
      {sub && <p style={{ color: '#6c7086', fontSize: '0.75rem', marginTop: '2px' }}>{sub}</p>}
    </div>
  );
}

export function DashboardConsolidated() {
  const currentYear = new Date().getFullYear();
  const currentMonth = new Date().getMonth() + 1;
  const [filter, setFilter] = useState<DashboardFilter>({ year: currentYear, month: currentMonth });

  const { data, isLoading, isError } = useConsolidatedDashboard(filter);

  const selectStyle = {
    padding: '6px 10px', borderRadius: '6px', border: '1px solid #45475a',
    background: '#313244', color: '#cdd6f4', fontSize: '0.85rem',
  };

  return (
    <div style={{ maxWidth: '1000px', margin: '0 auto' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1.25rem' }}>
        <h1 style={{ fontSize: '1.4rem', color: '#cba6f7', margin: 0 }}>Dashboard Consolidado</h1>

        {/* Filtros de período */}
        <div style={{ display: 'flex', gap: '0.5rem' }}>
          <select
            value={filter.year ?? ''}
            onChange={e => setFilter(f => ({ ...f, year: parseInt(e.target.value, 10) || undefined }))}
            style={selectStyle}
          >
            <option value="">Todos os anos</option>
            {[2024, 2025, 2026].map(y => <option key={y} value={y}>{y}</option>)}
          </select>
          <select
            value={filter.month ?? ''}
            onChange={e => setFilter(f => ({ ...f, month: parseInt(e.target.value, 10) || undefined }))}
            style={selectStyle}
          >
            <option value="">Todos os meses</option>
            {Array.from({ length: 12 }, (_, i) => (
              <option key={i + 1} value={i + 1}>
                {new Date(2000, i).toLocaleString('pt-BR', { month: 'long' })}
              </option>
            ))}
          </select>
        </div>
      </div>

      {isError && (
        <div style={{ color: '#f38ba8', padding: '1rem' }}>
          Sem permissão ou erro ao carregar dashboard. Verifique seu papel de acesso.
        </div>
      )}

      {isLoading && (
        <div style={{ color: '#a6adc8', textAlign: 'center', padding: '2rem' }}>Carregando…</div>
      )}

      {data && (
        <>
          {/* Métricas principais */}
          <div style={{ display: 'flex', gap: '1rem', flexWrap: 'wrap', marginBottom: '1.5rem' }}>
            <MetricCard
              label="Total de Vendas"
              value={centsToDisplay(data.totalSalesCents)}
              sub={`${data.orderCount} pedido${data.orderCount !== 1 ? 's' : ''}`}
              color="#a6e3a1"
            />
            <MetricCard
              label="Comissões Apuradas"
              value={centsToDisplay(data.totalCommCents)}
              color="#89b4fa"
            />
            <MetricCard
              label="Pendentes de Aprovação"
              value={centsToDisplay(data.pendingCommCents)}
              color="#fab387"
            />
            <MetricCard
              label="Aprovadas (a pagar)"
              value={centsToDisplay(data.approvedCommCents)}
              color="#89dceb"
            />
            <MetricCard
              label="Pagas"
              value={centsToDisplay(data.paidCommCents)}
              color="#a6e3a1"
            />
          </div>

          {/* Gráfico de vendedores */}
          {data.topVendors.length > 0 && (
            <div style={{
              background: '#181825', borderRadius: '12px', padding: '1.25rem',
              border: '1px solid #2e2e4a', marginBottom: '1.5rem',
            }}>
              <VendorBarChart vendorRankings={data.topVendors} />
            </div>
          )}

          {/* Ranking tabular */}
          <div style={{
            background: '#181825', borderRadius: '12px', padding: '1.25rem',
            border: '1px solid #2e2e4a',
          }}>
            <h2 style={{ color: '#a6adc8', fontSize: '0.95rem', marginBottom: '1rem' }}>
              Top Vendedores — {data.vendorCount} vendedore{data.vendorCount !== 1 ? 's' : ''}
            </h2>
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.875rem' }}>
              <thead>
                <tr>
                  {['#', 'Vendedor', 'Pedidos', 'Volume de Vendas', 'Comissões Net'].map(h => (
                    <th key={h} style={{
                      textAlign: 'left', padding: '6px 10px', color: '#6c7086',
                      borderBottom: '1px solid #2e2e4a', fontWeight: 500, fontSize: '0.78rem',
                    }}>{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {data.topVendors.map((v, i) => (
                  <tr key={v.vendorId} style={{ borderBottom: '1px solid #1e1e2e' }}>
                    <td style={{ padding: '8px 10px', color: '#6c7086' }}>{i + 1}</td>
                    <td style={{ padding: '8px 10px', color: '#cdd6f4' }}>{v.vendorName}</td>
                    <td style={{ padding: '8px 10px', color: '#a6adc8' }}>{v.orderCount}</td>
                    <td style={{ padding: '8px 10px', color: '#a6e3a1', fontWeight: 600 }}>
                      {centsToDisplay(v.totalCents)}
                    </td>
                    <td style={{ padding: '8px 10px', color: '#89b4fa', fontWeight: 600 }}>
                      {centsToDisplay(v.commCents)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}
    </div>
  );
}
