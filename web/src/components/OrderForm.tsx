// OrderForm — criar pedido com conversão decimal→centavos (Task 8.3.2)
// P-III: nunca tratar monetário como float; input string → int centavos ao enviar
import { useState } from 'react';
import { z } from 'zod';
import { useForm } from '../hooks/useForm';
import { useVendors } from '../api/vendors';
import type { CreateOrderRequest, CreateOrderItem } from '../api/orders';

// Converter "1500.00" → 150000 centavos (sem float intermediário)
// Usa manipulação de string para garantir aritmética inteira (CHK Task 8.3.4)
export function decimalToCents(value: string): number {
  const cleaned = value.trim().replace(',', '.');
  const dotIdx = cleaned.indexOf('.');
  if (dotIdx === -1) {
    return parseInt(cleaned, 10) * 100;
  }
  const intPart = cleaned.slice(0, dotIdx);
  const fracPart = cleaned.slice(dotIdx + 1).padEnd(2, '0').slice(0, 2);
  return parseInt(intPart, 10) * 100 + parseInt(fracPart, 10);
}

// Converter centavos → "R$ X,XX"
export function centsToDisplay(cents: number): string {
  return new Intl.NumberFormat('pt-BR', { style: 'currency', currency: 'BRL' })
    .format(cents / 100);
}

const orderFormSchema = z.object({
  vendorId: z.string().min(1, 'Vendedor obrigatório'),
  orderDate: z.string().min(1, 'Data obrigatória'),
});

type OrderFormData = z.infer<typeof orderFormSchema>;

interface OrderItem {
  description: string;
  quantity: string;
  unitPrice: string; // string decimal, ex: "1500.00"
}

interface OrderFormProps {
  onSubmit: (req: CreateOrderRequest) => void | Promise<void>;
  onCancel: () => void;
  isLoading?: boolean;
}

const inputStyle = {
  padding: '8px 12px', borderRadius: '6px', border: '1px solid #45475a',
  background: '#313244', color: '#cdd6f4', fontSize: '0.9rem',
  boxSizing: 'border-box' as const,
};

const labelStyle = { display: 'block', color: '#a6adc8', fontSize: '0.8rem', marginBottom: '4px' };

export function OrderForm({ onSubmit, onCancel, isLoading }: OrderFormProps) {
  const { data: vendors = [] } = useVendors();
  const activeVendors = vendors.filter(v => v.status === 'ativo');

  const form = useForm<OrderFormData>({
    schema: orderFormSchema,
    defaultValues: {
      vendorId: '',
      orderDate: new Date().toISOString().split('T')[0],
    },
  });

  const [items, setItems] = useState<OrderItem[]>([
    { description: '', quantity: '1', unitPrice: '' },
  ]);
  const [itemErrors, setItemErrors] = useState<string[]>([]);

  function updateItem(idx: number, field: keyof OrderItem, value: string) {
    setItems(prev => prev.map((it, i) => i === idx ? { ...it, [field]: value } : it));
  }

  function addItem() {
    setItems(prev => [...prev, { description: '', quantity: '1', unitPrice: '' }]);
  }

  function removeItem(idx: number) {
    setItems(prev => prev.filter((_, i) => i !== idx));
  }

  function validateItems(): CreateOrderItem[] | null {
    const errors: string[] = items.map(() => '');
    const result: CreateOrderItem[] = [];

    for (let i = 0; i < items.length; i++) {
      const it = items[i];
      if (!it.description.trim()) { errors[i] = 'Descrição obrigatória'; continue; }
      const qty = parseInt(it.quantity, 10);
      if (isNaN(qty) || qty <= 0) { errors[i] = 'Quantidade deve ser inteiro positivo'; continue; }
      if (!it.unitPrice.trim()) { errors[i] = 'Valor obrigatório'; continue; }
      const cents = decimalToCents(it.unitPrice);
      if (cents <= 0) { errors[i] = 'Valor deve ser positivo'; continue; }
      result.push({ description: it.description.trim(), quantity: qty, unitPriceCents: cents });
    }

    const hasError = errors.some(Boolean);
    setItemErrors(errors);
    return hasError ? null : result;
  }

  async function handleSubmit(data: OrderFormData) {
    const parsedItems = validateItems();
    if (!parsedItems) return;

    await onSubmit({
      vendorId: data.vendorId,
      orderDate: data.orderDate,
      items: parsedItems,
    });
  }

  // Preview total em centavos (sem float)
  const totalCents = items.reduce((sum, it) => {
    const cents = decimalToCents(it.unitPrice || '0');
    const qty = parseInt(it.quantity || '0', 10);
    return sum + (isNaN(qty) ? 0 : cents * qty);
  }, 0);

  return (
    <form onSubmit={form.handleSubmit(handleSubmit)} noValidate>
      <div style={{ display: 'flex', gap: '1rem', marginBottom: '1rem', flexWrap: 'wrap' }}>
        <div style={{ flex: 2, minWidth: '200px' }}>
          <label style={labelStyle} htmlFor="of-vendor">Vendedor *</label>
          <select
            id="of-vendor"
            style={{ ...inputStyle, width: '100%' }}
            value={form.values.vendorId}
            onChange={e => form.setValue('vendorId', e.target.value)}
          >
            <option value="">Selecione…</option>
            {activeVendors.map(v => (
              <option key={v.id} value={v.id}>{v.name}</option>
            ))}
          </select>
          {form.errors.vendorId && <p style={{ color: '#f38ba8', fontSize: '0.75rem' }}>{form.errors.vendorId}</p>}
        </div>
        <div style={{ flex: 1, minWidth: '160px' }}>
          <label style={labelStyle} htmlFor="of-date">Data do Pedido *</label>
          <input id="of-date" type="date" style={{ ...inputStyle, width: '100%' }} {...form.register('orderDate')} />
          {form.errors.orderDate && <p style={{ color: '#f38ba8', fontSize: '0.75rem' }}>{form.errors.orderDate}</p>}
        </div>
      </div>

      {/* Itens */}
      <div style={{ marginBottom: '1rem' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.75rem' }}>
          <span style={{ color: '#a6adc8', fontSize: '0.85rem', fontWeight: 600 }}>Itens do Pedido</span>
          <button type="button" onClick={addItem} style={{
            padding: '4px 10px', borderRadius: '6px', border: '1px solid #89b4fa',
            background: 'transparent', color: '#89b4fa', cursor: 'pointer', fontSize: '0.8rem',
          }}>
            + Item
          </button>
        </div>

        {items.map((it, idx) => (
          <div key={idx} style={{
            display: 'flex', gap: '0.75rem', alignItems: 'flex-start',
            marginBottom: '0.75rem', padding: '0.75rem', background: '#1e1e2e', borderRadius: '8px',
          }}>
            <div style={{ flex: 3 }}>
              <input
                style={{ ...inputStyle, width: '100%' }}
                placeholder="Descrição"
                value={it.description}
                onChange={e => updateItem(idx, 'description', e.target.value)}
              />
            </div>
            <div style={{ flex: 1, minWidth: '70px' }}>
              <input
                style={{ ...inputStyle, width: '100%' }}
                placeholder="Qtd"
                type="number"
                min="1"
                value={it.quantity}
                onChange={e => updateItem(idx, 'quantity', e.target.value)}
              />
            </div>
            <div style={{ flex: 2 }}>
              <input
                style={{ ...inputStyle, width: '100%' }}
                placeholder="Valor unit. (ex: 1500.00)"
                value={it.unitPrice}
                onChange={e => updateItem(idx, 'unitPrice', e.target.value)}
              />
            </div>
            {items.length > 1 && (
              <button type="button" onClick={() => removeItem(idx)} style={{
                background: 'transparent', border: 'none', color: '#f38ba8',
                cursor: 'pointer', fontSize: '1.1rem', marginTop: '6px',
              }}>
                ×
              </button>
            )}
            {itemErrors[idx] && (
              <p style={{ color: '#f38ba8', fontSize: '0.75rem', position: 'absolute', marginTop: '34px' }}>
                {itemErrors[idx]}
              </p>
            )}
          </div>
        ))}
      </div>

      {/* Total preview */}
      {totalCents > 0 && (
        <div style={{
          background: '#1a3b2b', borderRadius: '8px', padding: '0.75rem 1rem',
          marginBottom: '1rem', display: 'flex', justifyContent: 'space-between',
        }}>
          <span style={{ color: '#a6adc8', fontSize: '0.85rem' }}>Total estimado</span>
          <span style={{ color: '#a6e3a1', fontWeight: 700 }}>
            {centsToDisplay(totalCents)}
          </span>
        </div>
      )}

      <div style={{ display: 'flex', gap: '0.75rem', justifyContent: 'flex-end' }}>
        <button type="button" onClick={onCancel} style={{
          padding: '8px 16px', borderRadius: '6px', border: '1px solid #45475a',
          background: 'transparent', color: '#a6adc8', cursor: 'pointer',
        }}>
          Cancelar
        </button>
        <button type="submit" disabled={isLoading} style={{
          padding: '8px 16px', borderRadius: '6px', border: 'none',
          background: '#89b4fa', color: '#1e1e2e', fontWeight: 600,
          cursor: isLoading ? 'not-allowed' : 'pointer', opacity: isLoading ? 0.7 : 1,
        }}>
          {isLoading ? 'Criando…' : 'Criar Pedido'}
        </button>
      </div>
    </form>
  );
}
