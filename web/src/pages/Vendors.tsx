// Vendors — lista de vendedores com filtro por status e paginação (Task 8.2.1)
// Apenas Gestor/Financeiro (ProtectedRoute garante no roteador)
import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useVendors, useCreateVendor, useSetCommissionRule } from '../api/vendors';
import { VendorForm } from '../components/VendorForm';
import type { VendorStatus } from '../types/vendor';
import type { CreateVendorRequest } from '../types/vendor';

const PAGE_SIZE = 10;

function StatusBadge({ status }: { status: VendorStatus }) {
  return (
    <span style={{
      padding: '2px 10px',
      borderRadius: '9999px',
      fontSize: '0.75rem',
      fontWeight: 600,
      background: status === 'ativo' ? '#1a3b2b' : '#3b1a1a',
      color: status === 'ativo' ? '#a6e3a1' : '#f38ba8',
    }}>
      {status === 'ativo' ? 'Ativo' : 'Inativo'}
    </span>
  );
}

export function Vendors() {
  const { data: vendors = [], isLoading, isError } = useVendors();
  const createVendor = useCreateVendor();
  const [showForm, setShowForm] = useState(false);
  const [filterStatus, setFilterStatus] = useState<VendorStatus | 'todos'>('todos');
  const [page, setPage] = useState(0);

  const filtered = vendors.filter(v =>
    filterStatus === 'todos' ? true : v.status === filterStatus
  );
  const totalPages = Math.ceil(filtered.length / PAGE_SIZE);
  const paginated = filtered.slice(page * PAGE_SIZE, (page + 1) * PAGE_SIZE);

  async function handleCreate(vendorData: CreateVendorRequest, commission: { percentage: string; validFrom: string }) {
    const created = await createVendor.mutateAsync(vendorData);
    // Definir regra de comissão inicial
    if (created?.id) {
      const ruleHook = useSetCommissionRule;
      void ruleHook; // será chamado via api/vendors direto abaixo
      const { fetchJSON } = await import('../api/client');
      const { CommissionRuleSchema } = await import('../types/vendor.schema');
      await fetchJSON(`/vendors/${created.id}/commission-rule`, CommissionRuleSchema, {
        method: 'PUT',
        body: commission,
      });
    }
    setShowForm(false);
  }

  const cardStyle = {
    background: '#181825',
    borderRadius: '12px',
    padding: '1.5rem',
    marginBottom: '1rem',
  };

  if (isError) {
    return (
      <div style={{ color: '#f38ba8', padding: '1rem' }}>
        Erro ao carregar vendedores. Verifique suas permissões.
      </div>
    );
  }

  return (
    <div style={{ maxWidth: '900px', margin: '0 auto' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1.5rem' }}>
        <h1 style={{ fontSize: '1.4rem', color: '#cba6f7', margin: 0 }}>Vendedores</h1>
        <button
          onClick={() => setShowForm(true)}
          style={{
            padding: '8px 16px', borderRadius: '8px', border: 'none',
            background: '#a6e3a1', color: '#1e1e2e', fontWeight: 600, cursor: 'pointer',
          }}
        >
          + Novo Vendedor
        </button>
      </div>

      {/* Filtro por status */}
      <div style={{ display: 'flex', gap: '0.5rem', marginBottom: '1.25rem' }}>
        {(['todos', 'ativo', 'inativo'] as const).map(s => (
          <button key={s} onClick={() => { setFilterStatus(s); setPage(0); }} style={{
            padding: '5px 14px', borderRadius: '6px',
            border: '1px solid',
            borderColor: filterStatus === s ? '#cba6f7' : '#45475a',
            background: filterStatus === s ? '#3b2d5e' : 'transparent',
            color: filterStatus === s ? '#cba6f7' : '#a6adc8',
            cursor: 'pointer', fontSize: '0.85rem',
          }}>
            {s === 'todos' ? 'Todos' : s === 'ativo' ? 'Ativos' : 'Inativos'}
          </button>
        ))}
        <span style={{ marginLeft: 'auto', color: '#6c7086', fontSize: '0.85rem', alignSelf: 'center' }}>
          {filtered.length} vendedor{filtered.length !== 1 ? 'es' : ''}
        </span>
      </div>

      {/* Formulário de criação */}
      {showForm && (
        <div style={{ ...cardStyle, border: '1px solid #45475a', marginBottom: '1.5rem' }}>
          <h2 style={{ color: '#cdd6f4', marginBottom: '1rem', fontSize: '1.1rem' }}>Novo Vendedor</h2>
          <VendorForm
            onSubmit={(d, c) => handleCreate(d as CreateVendorRequest, c)}
            onCancel={() => setShowForm(false)}
            isLoading={createVendor.isPending}
          />
          {createVendor.error && (
            <p style={{ color: '#f38ba8', marginTop: '0.75rem', fontSize: '0.85rem' }}>
              Erro: {createVendor.error instanceof Error ? createVendor.error.message : 'Erro desconhecido'}
            </p>
          )}
        </div>
      )}

      {/* Lista */}
      {isLoading ? (
        <div style={{ color: '#a6adc8', textAlign: 'center', padding: '2rem' }}>Carregando…</div>
      ) : paginated.length === 0 ? (
        <div style={{ color: '#6c7086', textAlign: 'center', padding: '2rem' }}>
          Nenhum vendedor encontrado.
        </div>
      ) : (
        <div>
          {paginated.map(vendor => (
            <div key={vendor.id} style={{
              ...cardStyle,
              display: 'flex',
              alignItems: 'center',
              gap: '1rem',
              border: '1px solid #2e2e4a',
            }}>
              <div style={{ flex: 1 }}>
                <div style={{ fontWeight: 600, color: '#cdd6f4', marginBottom: '4px' }}>
                  {vendor.name}
                </div>
                <div style={{ color: '#a6adc8', fontSize: '0.85rem' }}>{vendor.email}</div>
                {vendor.anonymizedAt && (
                  <div style={{ color: '#f38ba8', fontSize: '0.75rem', marginTop: '3px' }}>
                    Anonimizado em {new Date(vendor.anonymizedAt).toLocaleDateString('pt-BR')}
                  </div>
                )}
              </div>
              <StatusBadge status={vendor.status} />
              <Link
                to={`/vendors/${vendor.id}`}
                style={{
                  padding: '6px 14px', borderRadius: '6px',
                  border: '1px solid #45475a', color: '#cba6f7',
                  textDecoration: 'none', fontSize: '0.85rem',
                }}
              >
                Detalhes
              </Link>
            </div>
          ))}
        </div>
      )}

      {/* Paginação */}
      {totalPages > 1 && (
        <div style={{ display: 'flex', gap: '0.5rem', justifyContent: 'center', marginTop: '1rem' }}>
          <button
            onClick={() => setPage(p => Math.max(0, p - 1))}
            disabled={page === 0}
            style={{ padding: '5px 12px', borderRadius: '6px', border: '1px solid #45475a',
              background: 'transparent', color: '#a6adc8', cursor: page === 0 ? 'not-allowed' : 'pointer' }}
          >
            ← Anterior
          </button>
          <span style={{ alignSelf: 'center', color: '#6c7086', fontSize: '0.85rem' }}>
            {page + 1} / {totalPages}
          </span>
          <button
            onClick={() => setPage(p => Math.min(totalPages - 1, p + 1))}
            disabled={page >= totalPages - 1}
            style={{ padding: '5px 12px', borderRadius: '6px', border: '1px solid #45475a',
              background: 'transparent', color: '#a6adc8', cursor: page >= totalPages - 1 ? 'not-allowed' : 'pointer' }}
          >
            Próxima →
          </button>
        </div>
      )}
    </div>
  );
}
