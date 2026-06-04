// API hooks para Vendedores — react-query + fetchJSON + Zod
// Task 7.2.2: useVendors, useVendor, useCreateVendor, useUpdateVendor, useDeactivateVendor
// Ref: contracts/api.md §/vendors; vendor.ts; vendor.schema.ts
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { fetchJSON } from './client';
import type { CommissionRule, CreateVendorRequest, Vendor, VendorPatch } from '../types/vendor';
import {
  CommissionRuleSchema,
  CreateVendorRequestSchema as _cvrs,
  VendorListSchema,
  VendorSchema,
} from '../types/vendor.schema';

// Suprimir aviso de variável não-usada — schema é importado para validação de tipos
void _cvrs;

// ─── Query keys ───────────────────────────────────────────────────────────────

export const vendorKeys = {
  all: ['vendors'] as const,
  list: () => [...vendorKeys.all, 'list'] as const,
  detail: (id: string) => [...vendorKeys.all, 'detail', id] as const,
  rule: (id: string) => [...vendorKeys.all, 'rule', id] as const,
};

// ─── Queries ──────────────────────────────────────────────────────────────────

/** GET /api/v1/vendors — lista todos os vendedores (Gestor/Financeiro).
 *  `enabled` permite ao chamador NÃO disparar a query quando o papel não tem
 *  permissão (ex.: Vendedor) — evitando um 403 inútil. */
export function useVendors(enabled = true) {
  return useQuery({
    queryKey: vendorKeys.list(),
    queryFn: () => fetchJSON('/vendors', VendorListSchema),
    enabled,
  });
}

/** GET /api/v1/vendors/{id} — detalhe de um vendedor */
export function useVendor(id: string) {
  return useQuery({
    queryKey: vendorKeys.detail(id),
    queryFn: () => fetchJSON(`/vendors/${id}`, VendorSchema),
    enabled: Boolean(id),
  });
}

/** GET /api/v1/vendors/{id}/commission-rule — taxa atual do vendedor */
export function useVendorCommissionRule(id: string) {
  return useQuery({
    queryKey: vendorKeys.rule(id),
    queryFn: () => fetchJSON(`/vendors/${id}/commission-rule`, CommissionRuleSchema),
    enabled: Boolean(id),
  });
}

// ─── Mutations ────────────────────────────────────────────────────────────────

/** POST /api/v1/vendors — criar vendedor (Gestor) */
export function useCreateVendor() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: CreateVendorRequest) =>
      fetchJSON('/vendors', VendorSchema, { method: 'POST', body: req }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: vendorKeys.list() });
    },
  });
}

/** PATCH /api/v1/vendors/{id} — atualizar vendedor (Gestor) */
export function useUpdateVendor(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (patch: VendorPatch) =>
      fetchJSON(`/vendors/${id}`, VendorSchema, { method: 'PATCH', body: patch }),
    onSuccess: (updated: Vendor) => {
      qc.setQueryData(vendorKeys.detail(id), updated);
      void qc.invalidateQueries({ queryKey: vendorKeys.list() });
    },
  });
}

/** DELETE /api/v1/vendors/{id} — anonimizar vendedor LGPD (Gestor) */
export function useDeactivateVendor() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      fetchJSON(`/vendors/${id}`, VendorSchema, { method: 'DELETE' }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: vendorKeys.all });
    },
  });
}

/** PUT /api/v1/vendors/{id}/commission-rule — definir taxa de comissão (Gestor) */
export function useSetCommissionRule(vendorId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: { percentage: string; validFrom: string }) =>
      fetchJSON(`/vendors/${vendorId}/commission-rule`, CommissionRuleSchema, {
        method: 'PUT',
        body: req,
      }),
    onSuccess: (rule: CommissionRule) => {
      qc.setQueryData(vendorKeys.rule(vendorId), rule);
    },
  });
}
