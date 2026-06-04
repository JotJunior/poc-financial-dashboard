// ApurationForm — acionar apuração de comissões por mês/ano (Task 8.4.2)
import { useState } from 'react';
import { useApurate } from '../api/commissions';
import { centsToDisplay } from './OrderForm';

export function ApurationForm() {
  const today = new Date();
  const [year, setYear] = useState(today.getFullYear());
  const [month, setMonth] = useState(today.getMonth() + 1);
  const apurate = useApurate();

  async function handleApurate() {
    await apurate.mutateAsync({ year, month });
  }

  const selectStyle = {
    padding: '7px 10px', borderRadius: '6px', border: '1px solid #45475a',
    background: '#313244', color: '#cdd6f4', fontSize: '0.9rem',
  };

  return (
    <div style={{
      background: '#181825', borderRadius: '12px', padding: '1.5rem',
      border: '1px solid #2e2e4a',
    }}>
      <h2 style={{ color: '#89dceb', margin: '0 0 1rem', fontSize: '1rem' }}>Apuração de Comissões</h2>

      <div style={{ display: 'flex', gap: '1rem', alignItems: 'flex-end', flexWrap: 'wrap' }}>
        <div>
          <label style={{ display: 'block', color: '#a6adc8', fontSize: '0.8rem', marginBottom: '4px' }}>
            Mês
          </label>
          <select value={month} onChange={e => setMonth(parseInt(e.target.value, 10))} style={selectStyle}>
            {Array.from({ length: 12 }, (_, i) => (
              <option key={i + 1} value={i + 1}>
                {new Date(2000, i).toLocaleString('pt-BR', { month: 'long' })}
              </option>
            ))}
          </select>
        </div>
        <div>
          <label style={{ display: 'block', color: '#a6adc8', fontSize: '0.8rem', marginBottom: '4px' }}>
            Ano
          </label>
          <select value={year} onChange={e => setYear(parseInt(e.target.value, 10))} style={selectStyle}>
            {[2024, 2025, 2026].map(y => <option key={y} value={y}>{y}</option>)}
          </select>
        </div>
        <button
          onClick={handleApurate}
          disabled={apurate.isPending}
          style={{
            padding: '8px 18px', borderRadius: '6px', border: 'none',
            background: '#89dceb', color: '#1e1e2e', fontWeight: 600,
            cursor: apurate.isPending ? 'not-allowed' : 'pointer',
            opacity: apurate.isPending ? 0.7 : 1,
          }}
        >
          {apurate.isPending ? 'Apurando…' : 'Apurar'}
        </button>
      </div>

      {/* Resultado */}
      {apurate.data && (
        <div style={{
          marginTop: '1.25rem', padding: '1rem', background: '#1a3b2b',
          borderRadius: '8px', border: '1px solid #a6e3a1',
        }}>
          <p style={{ color: '#a6e3a1', fontWeight: 600, marginBottom: '0.5rem' }}>
            Apuração concluída — {apurate.data.periodYear}/{String(apurate.data.periodMonth).padStart(2, '0')}
          </p>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: '0.75rem' }}>
            <div>
              <p style={{ color: '#6c7086', fontSize: '0.75rem', marginBottom: '2px' }}>Calculadas</p>
              <p style={{ color: '#a6e3a1', fontWeight: 700 }}>{apurate.data.calculated}</p>
            </div>
            <div>
              <p style={{ color: '#6c7086', fontSize: '0.75rem', marginBottom: '2px' }}>Puladas (idem.)</p>
              <p style={{ color: '#fab387', fontWeight: 700 }}>{apurate.data.skipped}</p>
            </div>
            <div>
              <p style={{ color: '#6c7086', fontSize: '0.75rem', marginBottom: '2px' }}>Total</p>
              <p style={{ color: '#a6e3a1', fontWeight: 700 }}>{centsToDisplay(apurate.data.totalCents)}</p>
            </div>
          </div>
        </div>
      )}

      {apurate.error && (
        <p style={{ color: '#f38ba8', marginTop: '0.75rem', fontSize: '0.85rem' }}>
          {apurate.error instanceof Error ? apurate.error.message : 'Erro ao apurar'}
        </p>
      )}
    </div>
  );
}
