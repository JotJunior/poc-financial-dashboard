// Tipos de Vendedor — espelha backend/internal/dto/vendor.go (camelCase)
// Task 7.1.1: Vendor, CreateVendorRequest, VendorPatch, CommissionRule
// Ref: contracts/api.md §/vendors; data-model.md §vendors; spec §FR-001..FR-005

export type VendorStatus = 'ativo' | 'inativo';

export interface Vendor {
  id: string; // UUID
  name: string; // máx 200 chars (CHK019)
  email: string; // máx 255 chars
  status: VendorStatus;
  anonymizedAt: string | null; // ISO 8601 UTC — LGPD (FR-005)
}

/** @deprecated Use CreateVendorRequest */
export interface VendorCreate {
  name: string;
  email: string;
}

export interface CreateVendorRequest {
  name: string;   // máx 200 chars
  email: string;  // máx 255 chars
  commissionPercentage?: string; // "5.5000" — obrigatório na API; opcional no tipo para compatibilidade
}

export interface VendorPatch {
  name?: string;
  email?: string;
  status?: VendorStatus;
}

/** @deprecated Use VendorPatch */
export interface VendorUpdate {
  name?: string;
  email?: string;
  status?: VendorStatus;
}

// CommissionRule — taxa de comissão de um vendedor em período
// Ref: contracts/api.md §/vendors/{id}/commission-rule; data-model.md §commission_rules
export interface CommissionRule {
  id: string;       // UUID
  vendorId: string;
  percentage: string; // "5.5000" — string para preservar precisão (P-III sem float)
  validFrom: string;  // ISO 8601 date
  validTo: string | null;
  version: number;
}

export interface SetCommissionRuleRequest {
  percentage: string; // "5.5000" — string para preservar precisão
  validFrom: string;  // ISO 8601 date "YYYY-MM-DD"
}
