// DashboardVendor — Vendedor: apenas dados próprios (Task 8.5.2)
// P-IV: backend sobrescreve vendorId do JWT; UI não permite selecionar outro
import { useVendorDashboard } from '../api/dashboard';
import { centsToDisplay } from '../components/OrderForm';

function MetricCard({ label, value, color = '#cdd6f4', sub }: {
  label: string; value: string; color?: string; sub?: string;
}) {
  return (
    <div style={{
      background: '#181825', borderRadius: '12px', padding: '1.25rem',
      border: '1px solid #2e2e4a', flex: 1, minWidth: '160px',
    }}>
      <p style={{ color: '#6c7086', fontSize: '0.75rem', marginBottom: '4px' }}>{label}</p>
      <p style={{ color, fontSize: '1.3rem', fontWeight: 700, margin: 0 }}>{value}</p>
      {sub && <p style={{ color: '#6c7086', fontSize: '0.75rem', marginTop: '2px' }}>{sub}</p>}
    </div>
  );
}

export function DashboardVendor() {
  // Backend usa vendorId do JWT — não passa vendorId na query para Vendedor
  const { data, isLoading, isError } = useVendorDashboard();

  if (isError) {
    return (
      <div style={{ color: '#f38ba8', padding: '1rem' }}>
        Erro ao carregar seu dashboard. Tente novamente.
      </div>
    );
  }

  return (
    <div style={{ maxWidth: '800px', margin: '0 auto' }}>
      <h1 style={{ fontSize: '1.4rem', color: '#89b4fa', margin: '0 0 1.25rem' }}>
        Meu Dashboard
      </h1>

      {isLoading ? (
        <div style={{ color: '#a6adc8', textAlign: 'center', padding: '2rem' }}>Carregando…</div>
      ) : data ? (
        <>
          <div style={{ display: 'flex', gap: '1rem', flexWrap: 'wrap', marginBottom: '1.5rem' }}>
            <MetricCard
              label="Volume de Vendas"
              value={centsToDisplay(data.totalSalesCents)}
              sub={`${data.orderCount} pedido${data.orderCount !== 1 ? 's' : ''}`}
              color="#a6e3a1"
            />
            <MetricCard
              label="Comissões Apuradas"
              value={centsToDisplay(data.totalCommCents)}
              color="#89b4fa"
            />
          </div>

          <div style={{ display: 'flex', gap: '1rem', flexWrap: 'wrap' }}>
            <MetricCard
              label="Pendentes de Aprovação"
              value={centsToDisplay(data.pendingCommCents)}
              color="#fab387"
            />
            <MetricCard
              label="Aprovadas (a receber)"
              value={centsToDisplay(data.approvedCommCents)}
              color="#89dceb"
            />
            <MetricCard
              label="Recebidas"
              value={centsToDisplay(data.paidCommCents)}
              color="#a6e3a1"
            />
          </div>

          <div style={{
            marginTop: '1.5rem', background: '#181825', borderRadius: '12px',
            padding: '1.25rem', border: '1px solid #2e2e4a',
          }}>
            <p style={{ color: '#6c7086', fontSize: '0.82rem' }}>
              Você está vendo apenas seus próprios dados.
              Para rastrear um pedido específico, navegue até a página de Pedidos.
            </p>
          </div>
        </>
      ) : null}
    </div>
  );
}
