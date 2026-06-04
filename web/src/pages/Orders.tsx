// Orders — lista com filtros, criação e transição de estado (Task 8.3.1)
// Exibe totalCents convertido para reais (P-III: nunca float)
import { useState } from 'react';
import { useOrders, useCreateOrder, useTransitionOrder } from '../api/orders';
import { useVendors } from '../api/vendors';
import { OrderForm, centsToDisplay } from '../components/OrderForm';
import { OrderStatusBadge } from '../components/OrderStatusBadge';
import { useAuth } from '../api/auth-context';
import type { OrderStatus } from '../types/order';
import type { CreateOrderRequest } from '../api/orders';

export function Orders() {
  const { user } = useAuth();
  const isGestor = user?.role === 'gestor';

  const [filterStatus, setFilterStatus] = useState<OrderStatus | ''>('');
  const [filterVendor, setFilterVendor] = useState('');
  const [filterYear, setFilterYear] = useState('');
  const [filterMonth, setFilterMonth] = useState('');
  const [showForm, setShowForm] = useState(false);

  const { data: orders = [], isLoading, isError } = useOrders({
    status: (filterStatus || undefined) as OrderStatus | undefined,
    vendorId: filterVendor || undefined,
    year: filterYear ? parseInt(filterYear, 10) : undefined,
    month: filterMonth ? parseInt(filterMonth, 10) : undefined,
  });
  const { data: vendors = [] } = useVendors();
  const createOrder = useCreateOrder();
  const transitionOrder = useTransitionOrder();

  const vendorName = (id: string) => vendors.find(v => v.id === id)?.name ?? id.slice(0, 8) + '…';

  async function handleCreate(req: CreateOrderRequest) {
    await createOrder.mutateAsync(req);
    setShowForm(false);
  }

  const card = {
    background: '#181825', borderRadius: '10px',
    padding: '1rem 1.25rem', border: '1px solid #2e2e4a', marginBottom: '0.75rem',
  };

  const selectStyle = {
    padding: '6px 10px', borderRadius: '6px', border: '1px solid #45475a',
    background: '#313244', color: '#cdd6f4', fontSize: '0.85rem',
  };

  if (isError) {
    return <div style={{ color: '#f38ba8', padding: '1rem' }}>Erro ao carregar pedidos.</div>;
  }

  return (
    <div style={{ maxWidth: '950px', margin: '0 auto' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1.25rem' }}>
        <h1 style={{ fontSize: '1.4rem', color: '#89b4fa', margin: 0 }}>Pedidos</h1>
        {isGestor && (
          <button onClick={() => setShowForm(true)} style={{
            padding: '8px 16px', borderRadius: '8px', border: 'none',
            background: '#89b4fa', color: '#1e1e2e', fontWeight: 600, cursor: 'pointer',
          }}>
            + Novo Pedido
          </button>
        )}
      </div>

      {/* Filtros */}
      <div style={{ display: 'flex', gap: '0.75rem', flexWrap: 'wrap', marginBottom: '1.25rem' }}>
        <select value={filterStatus} onChange={e => setFilterStatus(e.target.value as OrderStatus | '')} style={selectStyle}>
          <option value="">Todos os status</option>
          <option value="rascunho">Rascunho</option>
          <option value="confirmado">Confirmado</option>
          <option value="pago">Pago</option>
          <option value="cancelado">Cancelado</option>
        </select>

        {isGestor && (
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
          {orders.length} pedido{orders.length !== 1 ? 's' : ''}
        </span>
      </div>

      {/* Formulário de criação */}
      {showForm && isGestor && (
        <div style={{ ...card, marginBottom: '1.5rem' }}>
          <h2 style={{ color: '#cdd6f4', marginBottom: '1rem', fontSize: '1.05rem' }}>Novo Pedido</h2>
          <OrderForm
            onSubmit={handleCreate}
            onCancel={() => setShowForm(false)}
            isLoading={createOrder.isPending}
          />
          {createOrder.error && (
            <p style={{ color: '#f38ba8', marginTop: '0.75rem', fontSize: '0.85rem' }}>
              {createOrder.error instanceof Error ? createOrder.error.message : 'Erro ao criar pedido'}
            </p>
          )}
        </div>
      )}

      {/* Lista */}
      {isLoading ? (
        <div style={{ color: '#a6adc8', textAlign: 'center', padding: '2rem' }}>Carregando…</div>
      ) : orders.length === 0 ? (
        <div style={{ color: '#6c7086', textAlign: 'center', padding: '2rem' }}>
          Nenhum pedido encontrado.
        </div>
      ) : (
        orders.map(order => (
          <div key={order.id} style={card}>
            <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '1rem' }}>
              <div style={{ flex: 1 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', marginBottom: '4px' }}>
                  <code style={{ color: '#cba6f7', fontSize: '0.78rem' }}>{order.id.slice(0, 8)}…</code>
                  <span style={{ color: '#6c7086', fontSize: '0.8rem' }}>
                    {new Date(order.orderDate).toLocaleDateString('pt-BR')}
                  </span>
                </div>
                <div style={{ color: '#a6adc8', fontSize: '0.85rem', marginBottom: '6px' }}>
                  {vendorName(order.vendorId)}
                </div>
                <div style={{ color: '#a6e3a1', fontWeight: 700, fontSize: '1.05rem' }}>
                  {centsToDisplay(order.totalCents)}
                </div>
                {order.items.length > 0 && (
                  <div style={{ color: '#6c7086', fontSize: '0.78rem', marginTop: '3px' }}>
                    {order.items.length} item{order.items.length !== 1 ? 's' : ''}
                  </div>
                )}
              </div>
              <div>
                <OrderStatusBadge
                  status={order.status}
                  interactive={isGestor}
                  isLoading={transitionOrder.isPending}
                  onTransition={to => { void transitionOrder.mutateAsync({ id: order.id, status: to }); }}
                />
                {order.paidAt && (
                  <div style={{ color: '#6c7086', fontSize: '0.75rem', marginTop: '4px' }}>
                    Pago em {new Date(order.paidAt).toLocaleDateString('pt-BR')}
                  </div>
                )}
              </div>
            </div>
          </div>
        ))
      )}
    </div>
  );
}
