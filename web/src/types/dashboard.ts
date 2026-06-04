// Tipos de Dashboard — espelha backend/internal/dto/dashboard.go
// Task 7.1.4: ConsolidatedDashboard, VendorDashboard, DrillDown
// Ref: contracts/api.md §6 Dashboards; spec §FR-016..FR-019, SC-004
// P-III: valores monetários como number (inteiros centavos, nunca float)

// ─── Consolidated Dashboard (Gestor / Financeiro) ─────────────────────────────

export interface VendorRanking {
  vendorId: string;
  vendorName: string;
  totalCents: number; // pedidos pagos em centavos (P-III)
  orderCount: number;
  commCents: number; // comissões net em centavos (P-III)
}

export interface ConsolidatedDashboard {
  totalSalesCents: number;   // P-III
  totalCommCents: number;    // P-III
  pendingCommCents: number;  // P-III
  approvedCommCents: number; // P-III
  paidCommCents: number;     // P-III
  orderCount: number;
  vendorCount: number;
  topVendors: VendorRanking[];
}

// ─── Vendor Dashboard (Vendedor — apenas os próprios; Gestor com ?vendorId) ──

export interface VendorDashboard {
  vendorId: string;
  totalSalesCents: number;   // P-III
  totalCommCents: number;    // P-III
  pendingCommCents: number;  // P-III
  approvedCommCents: number; // P-III
  paidCommCents: number;     // P-III
  orderCount: number;
}

// ─── Pending Commissions Summary (Gestor / Financeiro) ──────────────────────

export interface PendingCommissions {
  pendingApprovalCount: number;
  pendingApprovalCents: number; // P-III
  approvedUnpaidCount: number;
  approvedUnpaidCents: number;  // P-III
}

// ─── DrillDown (SC-004: rastreabilidade pedido → comissão → estorno) ─────────

export interface DrillDownReversal {
  reversalId: string;
  valueCents: number; // negativo (P-III)
  status: string;
  createdAt: string; // ISO 8601 UTC
  actorUserId: string; // auditabilidade P-I
}

export interface DrillDownCommission {
  commissionId: string;
  valueCents: number; // bruto (P-III)
  netCents: number;   // após estornos (P-III)
  status: string;
  periodYear: number;
  periodMonth: number;
  reversals: DrillDownReversal[];
}

export interface DrillDown {
  orderId: string;
  vendorId: string;
  totalCents: number; // P-III
  orderDate: string;  // "YYYY-MM-DD"
  status: string;
  createdAt: string; // ISO 8601 UTC
  commissions: DrillDownCommission[];
}

// ─── Dashboard Filter ─────────────────────────────────────────────────────────

export interface DashboardFilter {
  year?: number;
  month?: number;
  vendorId?: string; // apenas para Gestor/Financeiro
}
