// Barrel export para types do Financial Dashboard
// Task 7.1.6: todos os tipos e schemas Zod exportados por aqui
// Convenção: valores monetários = number (centavos int64 serializado como number JS)
// Percentuais = string ("5.5000") para preservar precisão (P-III sem float)

// ─── Tipos de domínio ─────────────────────────────────────────────────────────
export * from './auth';
export * from './vendor';
export * from './order';
export * from './commission';
export * from './dashboard';

// ─── Schemas Zod (borda de validação — CHK026) ────────────────────────────────
export * from './auth.schema';
export * from './vendor.schema';
export * from './order.schema';
export * from './commission.schema';
export * from './dashboard.schema';
