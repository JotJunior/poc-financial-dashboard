// Tipos de Comissão e Estorno — espelha backend/internal/dto/commission.go
// Ref: contracts/api.md §/commissions; spec §FR-020, FR-027-029

export type CommissionStatus = 'pendente' | 'aprovado' | 'pago';
export type ReversalStatus = 'aplicado' | 'pendente_aprovacao' | 'aprovado' | 'lancado';

export interface Commission {
  id: string; // UUID
  orderId: string;
  vendorId: string;
  valueCents: number; // int64 centavos (P-III)
  appliedPercentage: string; // "5.5000" — decimal string (P-III sem float)
  ruleId: string;
  periodYear: number;
  periodMonth: number; // 1-12
  status: CommissionStatus;
  calculatedAt: string; // ISO 8601 UTC
}

export interface CommissionReversal {
  id: string; // UUID
  commissionId: string;
  orderId: string;
  valueCents: number; // negativo (≤ 0) — estorno reduz saldo
  status: ReversalStatus;
  createdAt: string; // ISO 8601 UTC
  actorUserId: string;
}

export interface CommissionNetBalance {
  commissionId: string;
  vendorId: string;
  netCents: number; // saldo líquido = commission.value_cents + SUM(reversals.value_cents)
}
