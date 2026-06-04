// Tipos de Comissão e Estorno — espelha backend/internal/dto/commission.go
// Task 7.1.3: Commission, CommissionReversal, ApurationRequest, ApurationResult, CommissionNetBalance
// Ref: contracts/api.md §/commissions; spec §FR-020, FR-027-029; P-III (sem float)

export type CommissionStatus = 'pendente' | 'aprovado' | 'pago';
export type ReversalStatus = 'aplicado' | 'pendente_aprovacao' | 'aprovado' | 'lancado';

export interface Commission {
  id: string; // UUID
  orderId: string;
  vendorId: string;
  valueCents: number; // int64 centavos (P-III — nunca float)
  netCents: number;   // líquido após estornos (P-III)
  appliedPercentage: string; // "5.5000" — string para preservar precisão (P-III)
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
  valueCents: number; // negativo (≤ 0) — estorno reduz saldo (P-III)
  status: ReversalStatus;
  createdAt: string; // ISO 8601 UTC
  actorUserId: string; // auditabilidade P-I
}

export interface CommissionNetBalance {
  commissionId: string;
  vendorId: string;
  netCents: number; // saldo líquido = value_cents + SUM(reversals.value_cents) (P-III)
}

// ApurationRequest — corpo para POST /commissions/apurate
export interface ApurationRequest {
  year: number;
  month: number; // 1-12
}

// ApurationResult — resposta de POST /commissions/apurate
export interface ApurationResult {
  periodYear: number;
  periodMonth: number;
  calculated: number; // comissões calculadas/inseridas
  skipped: number;    // comissões puladas por idempotência (FR-014)
  totalCents: number; // soma das comissões inseridas (P-III — nunca float)
}

// TransitionCommissionRequest — corpo para PATCH /commissions/{id}/status
export interface TransitionCommissionRequest {
  status: CommissionStatus;
  motivo?: string; // obrigatório ao reverter aprovado→pendente
}
