// VendorForm — criar/editar vendedor com regra de comissão (Task 8.2.2)
// Validação Zod: percentual [0,100] exibido como "5,50%" string decimal
import { z } from 'zod';
import { useForm } from '../hooks/useForm';
import type { Vendor, VendorPatch, CreateVendorRequest } from '../types/vendor';

const vendorFormSchema = z.object({
  name: z.string().min(1, 'Nome obrigatório').max(200, 'Máximo 200 caracteres'),
  email: z.string().email('E-mail inválido').max(255, 'Máximo 255 caracteres'),
  percentage: z
    .string()
    .min(1, 'Percentual obrigatório')
    .refine(v => {
      const n = parseFloat(v.replace(',', '.'));
      return !isNaN(n) && n >= 0 && n <= 100;
    }, 'Percentual deve ser entre 0 e 100'),
  validFrom: z.string().min(1, 'Data de vigência obrigatória'),
});

type VendorFormData = z.infer<typeof vendorFormSchema>;

interface VendorFormProps {
  vendor?: Vendor;
  onSubmit: (data: CreateVendorRequest | VendorPatch, commission: { percentage: string; validFrom: string }) => void | Promise<void>;
  onCancel: () => void;
  isLoading?: boolean;
}

const inputStyle = {
  width: '100%',
  padding: '9px 12px',
  borderRadius: '8px',
  border: '1px solid #45475a',
  background: '#313244',
  color: '#cdd6f4',
  fontSize: '0.9rem',
  boxSizing: 'border-box' as const,
};

const labelStyle = {
  display: 'block',
  marginBottom: '5px',
  fontSize: '0.82rem',
  color: '#a6adc8',
  fontWeight: 500,
};

const errorStyle = { color: '#f38ba8', fontSize: '0.78rem', marginTop: '3px' };

export function VendorForm({ vendor, onSubmit, onCancel, isLoading }: VendorFormProps) {
  const today = new Date().toISOString().split('T')[0];

  const form = useForm<VendorFormData>({
    schema: vendorFormSchema,
    defaultValues: {
      name: vendor?.name ?? '',
      email: vendor?.email ?? '',
      percentage: '',
      validFrom: today,
    },
  });

  async function handleSubmit(data: VendorFormData) {
    const normalizedPct = data.percentage.replace(',', '.');
    const commission = {
      percentage: parseFloat(normalizedPct).toFixed(4),
      validFrom: data.validFrom,
    };
    const vendorData = vendor
      ? ({ name: data.name, email: data.email } as VendorPatch)
      : ({ name: data.name, email: data.email } as CreateVendorRequest);
    await onSubmit(vendorData, commission);
  }

  return (
    <form onSubmit={form.handleSubmit(handleSubmit)} noValidate>
      <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>

        <div>
          <label style={labelStyle} htmlFor="vf-name">Nome *</label>
          <input id="vf-name" style={inputStyle} {...form.register('name')} />
          {form.errors.name && <p style={errorStyle}>{form.errors.name}</p>}
        </div>

        <div>
          <label style={labelStyle} htmlFor="vf-email">E-mail *</label>
          <input id="vf-email" type="email" style={inputStyle} {...form.register('email')} />
          {form.errors.email && <p style={errorStyle}>{form.errors.email}</p>}
        </div>

        <div>
          <label style={labelStyle} htmlFor="vf-pct">
            Percentual de comissão (ex: 5,50) *
          </label>
          <div style={{ position: 'relative' }}>
            <input
              id="vf-pct"
              style={{ ...inputStyle, paddingRight: '32px' }}
              placeholder="5,50"
              {...form.register('percentage')}
            />
            <span style={{ position: 'absolute', right: '10px', top: '50%', transform: 'translateY(-50%)', color: '#a6adc8' }}>
              %
            </span>
          </div>
          {form.errors.percentage && <p style={errorStyle}>{form.errors.percentage}</p>}
          <p style={{ color: '#6c7086', fontSize: '0.75rem', marginTop: '3px' }}>
            Aceita ponto ou vírgula. Ex: 5.5 ou 5,5
          </p>
        </div>

        <div>
          <label style={labelStyle} htmlFor="vf-from">Vigência a partir de *</label>
          <input id="vf-from" type="date" style={inputStyle} {...form.register('validFrom')} />
          {form.errors.validFrom && <p style={errorStyle}>{form.errors.validFrom}</p>}
        </div>

        <div style={{ display: 'flex', gap: '0.75rem', justifyContent: 'flex-end', marginTop: '0.5rem' }}>
          <button type="button" onClick={onCancel} style={{
            padding: '8px 16px', borderRadius: '6px', border: '1px solid #45475a',
            background: 'transparent', color: '#a6adc8', cursor: 'pointer',
          }}>
            Cancelar
          </button>
          <button type="submit" disabled={isLoading} style={{
            padding: '8px 16px', borderRadius: '6px', border: 'none',
            background: '#a6e3a1', color: '#1e1e2e', fontWeight: 600,
            cursor: isLoading ? 'not-allowed' : 'pointer', opacity: isLoading ? 0.7 : 1,
          }}>
            {isLoading ? 'Salvando…' : vendor ? 'Atualizar' : 'Criar Vendedor'}
          </button>
        </div>
      </div>
    </form>
  );
}
