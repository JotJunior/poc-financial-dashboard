// SalesChart — gráfico de linha (vendas) + barras (comissões) com recharts (Task 8.5.3)
// P-III: valores sempre em centavos, formatados ao exibir
import {
  LineChart, Line, BarChart, Bar,
  XAxis, YAxis, CartesianGrid, Tooltip,
  ResponsiveContainer, Legend,
} from 'recharts';
import type { VendorRanking } from '../types/dashboard';

interface SalesChartProps {
  /** Dados de vendas por período (ex: agrupados por mês via topVendors) */
  vendorRankings: VendorRanking[];
}

function centsLabel(value: unknown): string {
  if (typeof value !== 'number') return String(value);
  return new Intl.NumberFormat('pt-BR', { style: 'currency', currency: 'BRL' })
    .format(value / 100);
}

const CHART_COLORS = ['#cba6f7', '#89b4fa', '#a6e3a1', '#fab387', '#89dceb', '#f38ba8'];

export function VendorBarChart({ vendorRankings }: SalesChartProps) {
  const data = vendorRankings.slice(0, 10).map(v => ({
    name: v.vendorName.length > 12 ? v.vendorName.slice(0, 12) + '…' : v.vendorName,
    vendas: v.totalCents,
    comissoes: v.commCents,
  }));

  if (data.length === 0) {
    return (
      <div style={{ textAlign: 'center', color: '#6c7086', padding: '2rem' }}>
        Sem dados para exibir.
      </div>
    );
  }

  return (
    <div>
      <h3 style={{ color: '#a6adc8', fontSize: '0.9rem', marginBottom: '0.75rem', fontWeight: 500 }}>
        Ranking de Vendedores (Volume de Vendas)
      </h3>
      <ResponsiveContainer width="100%" height={280}>
        <BarChart data={data} margin={{ top: 5, right: 20, left: 10, bottom: 50 }}>
          <CartesianGrid strokeDasharray="3 3" stroke="#2e2e4a" />
          <XAxis
            dataKey="name"
            tick={{ fill: '#a6adc8', fontSize: 11 }}
            angle={-30}
            textAnchor="end"
          />
          <YAxis
            tickFormatter={v => `R$${(v / 100000).toFixed(0)}k`}
            tick={{ fill: '#a6adc8', fontSize: 11 }}
          />
          <Tooltip
            formatter={(value: unknown) => centsLabel(value)}
            contentStyle={{ background: '#1e1e2e', border: '1px solid #45475a', borderRadius: '8px' }}
            labelStyle={{ color: '#cdd6f4' }}
          />
          <Legend wrapperStyle={{ color: '#a6adc8', fontSize: '0.8rem' }} />
          <Bar dataKey="vendas" name="Vendas" fill={CHART_COLORS[0]} radius={[4, 4, 0, 0]} />
          <Bar dataKey="comissoes" name="Comissões" fill={CHART_COLORS[1]} radius={[4, 4, 0, 0]} />
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}

interface CommissionLineChartProps {
  /** Dados de comissões no tempo — gerado a partir dos filtros de período */
  data: Array<{ label: string; pendenteCents: number; aprovadoCents: number; pagoCents: number }>;
}

export function CommissionLineChart({ data }: CommissionLineChartProps) {
  if (data.length === 0) {
    return (
      <div style={{ textAlign: 'center', color: '#6c7086', padding: '2rem' }}>
        Sem dados para exibir.
      </div>
    );
  }

  return (
    <div>
      <h3 style={{ color: '#a6adc8', fontSize: '0.9rem', marginBottom: '0.75rem', fontWeight: 500 }}>
        Evolução de Comissões por Período
      </h3>
      <ResponsiveContainer width="100%" height={240}>
        <LineChart data={data} margin={{ top: 5, right: 20, left: 10, bottom: 5 }}>
          <CartesianGrid strokeDasharray="3 3" stroke="#2e2e4a" />
          <XAxis dataKey="label" tick={{ fill: '#a6adc8', fontSize: 11 }} />
          <YAxis
            tickFormatter={v => `R$${(v / 100000).toFixed(0)}k`}
            tick={{ fill: '#a6adc8', fontSize: 11 }}
          />
          <Tooltip
            formatter={(value: unknown) => centsLabel(value)}
            contentStyle={{ background: '#1e1e2e', border: '1px solid #45475a', borderRadius: '8px' }}
            labelStyle={{ color: '#cdd6f4' }}
          />
          <Legend wrapperStyle={{ color: '#a6adc8', fontSize: '0.8rem' }} />
          <Line type="monotone" dataKey="pendenteCents" name="Pendentes" stroke={CHART_COLORS[3]} strokeWidth={2} dot={false} />
          <Line type="monotone" dataKey="aprovadoCents" name="Aprovados" stroke={CHART_COLORS[1]} strokeWidth={2} dot={false} />
          <Line type="monotone" dataKey="pagoCents" name="Pagos" stroke={CHART_COLORS[2]} strokeWidth={2} dot={false} />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}
