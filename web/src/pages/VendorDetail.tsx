// VendorDetail — histórico de regras, desativar, anonimizar LGPD (Task 8.2.3)
// 403 tratado explicitamente (Task 8.2.4)
import { useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useVendor, useVendorCommissionRule, useUpdateVendor, useDeactivateVendor, useSetCommissionRule } from '../api/vendors';
import { useAuth } from '../api/auth-context';
import { ApiError } from '../api/client';
import { useForm } from '../hooks/useForm';
import { centsToDisplay as _centsToDisplay } from '../components/OrderForm';
void _centsToDisplay;
import { z } from 'zod';

const newRuleSchema = z.object({
  percentage: z.string().refine(v => {
    const n = parseFloat(v.replace(',', '.'));
    return !isNaN(n) && n >= 0 && n <= 100;
  }, 'Percentual deve ser entre 0 e 100'),
  validFrom: z.string().min(1, 'Data obrigatória'),
});

type NewRuleForm = z.infer<typeof newRuleSchema>;

export function VendorDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { user } = useAuth();
  const isGestor = user?.role === 'gestor';

  const { data: vendor, isLoading, error } = useVendor(id ?? '');
  const { data: currentRule } = useVendorCommissionRule(id ?? '');
  const updateVendor = useUpdateVendor(id ?? '');
  const deactivateVendor = useDeactivateVendor();
  const setRule = useSetCommissionRule(id ?? '');

  const [showNewRule, setShowNewRule] = useState(false);
  const [confirmAnon, setConfirmAnon] = useState(false);

  const ruleForm = useForm<NewRuleForm>({
    schema: newRuleSchema,
    defaultValues: { percentage: '', validFrom: new Date().toISOString().split('T')[0] },
  });

  // 403 — sem permissão
  if (error instanceof ApiError && error.status === 403) {
    return (
      <div style={{ padding: '2rem', color: '#f38ba8' }}>
        Sem permissão para acessar este vendedor.
      </div>
    );
  }

  if (isLoading) {
    return <div style={{ color: '#a6adc8', padding: '2rem' }}>Carregando…</div>;
  }

  if (!vendor) {
    return <div style={{ color: '#f38ba8', padding: '2rem' }}>Vendedor não encontrado.</div>;
  }

  async function handleDeactivate() {
    if (!confirm(`Desativar ${vendor!.name}?`)) return;
    await updateVendor.mutateAsync({ status: 'inativo' });
  }

  async function handleAnonymize() {
    await deactivateVendor.mutateAsync(id!);
    navigate('/vendors');
  }

  async function handleNewRule(data: NewRuleForm) {
    await setRule.mutateAsync({
      percentage: parseFloat(data.percentage.replace(',', '.')).toFixed(4),
      validFrom: data.validFrom,
    });
    setShowNewRule(false);
    ruleForm.reset();
  }

  const card = {
    background: '#181825',
    borderRadius: '12px',
    padding: '1.5rem',
    border: '1px solid #2e2e4a',
    marginBottom: '1rem',
  };

  const inputStyle = {
    padding: '8px 12px', borderRadius: '6px', border: '1px solid #45475a',
    background: '#313244', color: '#cdd6f4', fontSize: '0.9rem',
  };

  return (
    <div style={{ maxWidth: '750px', margin: '0 auto' }}>
      {/* Breadcrumb */}
      <div style={{ marginBottom: '1rem' }}>
        <button onClick={() => navigate('/vendors')} style={{
          background: 'transparent', border: 'none', color: '#cba6f7',
          cursor: 'pointer', fontSize: '0.85rem',
        }}>
          ← Vendedores
        </button>
      </div>

      {/* Header */}
      <div style={{ ...card, display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between' }}>
        <div>
          <h1 style={{ color: '#cdd6f4', margin: '0 0 0.25rem' }}>{vendor.name}</h1>
          <p style={{ color: '#a6adc8', margin: '0 0 0.5rem', fontSize: '0.9rem' }}>{vendor.email}</p>
          <span style={{
            padding: '2px 10px', borderRadius: '9999px', fontSize: '0.75rem', fontWeight: 600,
            background: vendor.status === 'ativo' ? '#1a3b2b' : '#3b1a1a',
            color: vendor.status === 'ativo' ? '#a6e3a1' : '#f38ba8',
          }}>
            {vendor.status === 'ativo' ? 'Ativo' : 'Inativo'}
          </span>
          {vendor.anonymizedAt && (
            <p style={{ color: '#f38ba8', fontSize: '0.75rem', marginTop: '4px' }}>
              Anonimizado em {new Date(vendor.anonymizedAt).toLocaleDateString('pt-BR')}
            </p>
          )}
        </div>

        {isGestor && !vendor.anonymizedAt && (
          <div style={{ display: 'flex', gap: '0.5rem' }}>
            {vendor.status === 'ativo' && (
              <button onClick={handleDeactivate} disabled={updateVendor.isPending} style={{
                padding: '7px 14px', borderRadius: '6px', border: '1px solid #fab387',
                background: 'transparent', color: '#fab387', cursor: 'pointer', fontSize: '0.85rem',
              }}>
                Desativar
              </button>
            )}
            <button onClick={() => setConfirmAnon(true)} style={{
              padding: '7px 14px', borderRadius: '6px', border: '1px solid #f38ba8',
              background: 'transparent', color: '#f38ba8', cursor: 'pointer', fontSize: '0.85rem',
            }}>
              LGPD — Anonimizar
            </button>
          </div>
        )}
      </div>

      {/* Confirmação LGPD */}
      {confirmAnon && (
        <div style={{ ...card, background: '#3b0000', borderColor: '#f38ba8' }}>
          <p style={{ color: '#f38ba8', marginBottom: '1rem' }}>
            Esta ação é <strong>irreversível</strong>. Os dados pessoais ({vendor.name}, {vendor.email})
            serão anonimizados conforme LGPD. Os registros financeiros serão preservados.
          </p>
          <div style={{ display: 'flex', gap: '0.75rem' }}>
            <button onClick={() => setConfirmAnon(false)} style={{
              padding: '7px 14px', borderRadius: '6px', border: '1px solid #45475a',
              background: 'transparent', color: '#a6adc8', cursor: 'pointer',
            }}>
              Cancelar
            </button>
            <button onClick={handleAnonymize} disabled={deactivateVendor.isPending} style={{
              padding: '7px 14px', borderRadius: '6px', border: 'none',
              background: '#f38ba8', color: '#1e1e2e', fontWeight: 700, cursor: 'pointer',
            }}>
              {deactivateVendor.isPending ? 'Anonimizando…' : 'Confirmar Anonimização'}
            </button>
          </div>
        </div>
      )}

      {/* Regra de Comissão Atual */}
      <div style={card}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1rem' }}>
          <h2 style={{ color: '#89b4fa', margin: 0, fontSize: '1rem' }}>Regra de Comissão Atual</h2>
          {isGestor && (
            <button onClick={() => setShowNewRule(v => !v)} style={{
              padding: '5px 12px', borderRadius: '6px', border: '1px solid #89b4fa',
              background: 'transparent', color: '#89b4fa', cursor: 'pointer', fontSize: '0.82rem',
            }}>
              {showNewRule ? 'Cancelar' : '+ Nova Regra'}
            </button>
          )}
        </div>

        {currentRule ? (
          <div>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: '1rem' }}>
              <div>
                <p style={{ color: '#6c7086', fontSize: '0.75rem', marginBottom: '2px' }}>Percentual</p>
                <p style={{ color: '#a6e3a1', fontWeight: 700, fontSize: '1.2rem' }}>
                  {parseFloat(currentRule.percentage).toLocaleString('pt-BR', { minimumFractionDigits: 2 })}%
                </p>
              </div>
              <div>
                <p style={{ color: '#6c7086', fontSize: '0.75rem', marginBottom: '2px' }}>Versão</p>
                <p style={{ color: '#cdd6f4', fontWeight: 600 }}>v{currentRule.version}</p>
              </div>
              <div>
                <p style={{ color: '#6c7086', fontSize: '0.75rem', marginBottom: '2px' }}>Vigência</p>
                <p style={{ color: '#cdd6f4', fontSize: '0.85rem' }}>
                  {new Date(currentRule.validFrom).toLocaleDateString('pt-BR')}
                  {currentRule.validTo ? ` → ${new Date(currentRule.validTo).toLocaleDateString('pt-BR')}` : ' → atual'}
                </p>
              </div>
            </div>
          </div>
        ) : (
          <p style={{ color: '#6c7086' }}>Nenhuma regra definida.</p>
        )}

        {/* Formulário nova regra */}
        {showNewRule && (
          <form onSubmit={ruleForm.handleSubmit(handleNewRule)} style={{ marginTop: '1.25rem', borderTop: '1px solid #2e2e4a', paddingTop: '1rem' }}>
            <div style={{ display: 'flex', gap: '1rem', flexWrap: 'wrap' }}>
              <div style={{ flex: 1, minWidth: '150px' }}>
                <label style={{ display: 'block', color: '#a6adc8', fontSize: '0.8rem', marginBottom: '4px' }}>
                  Novo percentual *
                </label>
                <input style={inputStyle} placeholder="5,50" {...ruleForm.register('percentage')} />
                {ruleForm.errors.percentage && <p style={{ color: '#f38ba8', fontSize: '0.75rem' }}>{ruleForm.errors.percentage}</p>}
              </div>
              <div style={{ flex: 1, minWidth: '150px' }}>
                <label style={{ display: 'block', color: '#a6adc8', fontSize: '0.8rem', marginBottom: '4px' }}>
                  Vigência a partir de *
                </label>
                <input type="date" style={inputStyle} {...ruleForm.register('validFrom')} />
                {ruleForm.errors.validFrom && <p style={{ color: '#f38ba8', fontSize: '0.75rem' }}>{ruleForm.errors.validFrom}</p>}
              </div>
            </div>
            <button type="submit" disabled={setRule.isPending} style={{
              marginTop: '1rem', padding: '8px 16px', borderRadius: '6px', border: 'none',
              background: '#a6e3a1', color: '#1e1e2e', fontWeight: 600, cursor: 'pointer',
            }}>
              {setRule.isPending ? 'Salvando…' : 'Salvar Nova Regra'}
            </button>
          </form>
        )}
      </div>

      {/* Histórico de auditabilidade */}
      <div style={card}>
        <h2 style={{ color: '#89b4fa', margin: '0 0 0.75rem', fontSize: '1rem' }}>
          Auditabilidade (FR-003)
        </h2>
        <p style={{ color: '#6c7086', fontSize: '0.85rem' }}>
          Regra de comissão atual: v{currentRule?.version ?? '—'}.
          Versões anteriores são imutáveis (trigger P-I no banco).
          Histórico completo disponível via audit_trail.
        </p>
        <p style={{ color: '#6c7086', fontSize: '0.8rem', marginTop: '0.5rem' }}>
          ID do vendedor: <code style={{ color: '#cba6f7' }}>{vendor.id}</code>
        </p>
      </div>
    </div>
  );
}
