// Tipos de Vendedor — espelha backend/internal/dto/vendor.go (camelCase)
// Ref: contracts/api.md §/vendors; data-model.md §vendors

export type VendorStatus = 'ativo' | 'inativo';

export interface Vendor {
  id: string; // UUID
  name: string; // máx 200 chars (CHK019)
  email: string; // máx 255 chars
  status: VendorStatus;
  anonymizedAt: string | null; // ISO 8601 UTC — LGPD (FR-005)
}

export interface VendorCreate {
  name: string;
  email: string;
}

export interface VendorUpdate {
  name?: string;
  email?: string;
  status?: VendorStatus;
}
