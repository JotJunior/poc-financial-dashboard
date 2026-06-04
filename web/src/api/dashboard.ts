// API hooks para Dashboard — react-query + fetchJSON + Zod
// Task 7.2.5: useConsolidatedDashboard, useVendorDashboard, usePendingCommissions, useDrillDown
// Ref: contracts/api.md §6 Dashboards; dashboard.ts; dashboard.schema.ts
import { useQuery } from '@tanstack/react-query';

import { fetchJSON } from './client';
import type { DashboardFilter } from '../types/dashboard';
import {
  ConsolidatedDashboardSchema,
  DrillDownSchema,
  PendingCommissionsSchema,
  VendorDashboardSchema,
} from '../types/dashboard.schema';

// ─── Helpers ──────────────────────────────────────────────────────────────────

function buildDashboardQuery(filter?: DashboardFilter): string {
  if (!filter) return '';
  const params = new URLSearchParams();
  if (filter.year) params.set('year', String(filter.year));
  if (filter.month) params.set('month', String(filter.month));
  if (filter.vendorId) params.set('vendorId', filter.vendorId);
  const q = params.toString();
  return q ? `?${q}` : '';
}

// ─── Query keys ───────────────────────────────────────────────────────────────

export const dashboardKeys = {
  all: ['dashboard'] as const,
  consolidated: (filter?: DashboardFilter) =>
    [...dashboardKeys.all, 'consolidated', filter ?? {}] as const,
  vendor: (filter?: DashboardFilter) =>
    [...dashboardKeys.all, 'vendor', filter ?? {}] as const,
  pending: () => [...dashboardKeys.all, 'pending'] as const,
  drilldown: (orderId: string) => [...dashboardKeys.all, 'drilldown', orderId] as const,
};

// ─── Queries ──────────────────────────────────────────────────────────────────

/**
 * GET /api/v1/dashboard/consolidated — métricas agregadas (Gestor/Financeiro).
 * P-IV: 403 se Vendedor chamar — tratar no componente.
 */
export function useConsolidatedDashboard(filter?: DashboardFilter) {
  return useQuery({
    queryKey: dashboardKeys.consolidated(filter),
    queryFn: () =>
      fetchJSON(
        `/dashboard/consolidated${buildDashboardQuery(filter)}`,
        ConsolidatedDashboardSchema,
      ),
  });
}

/**
 * GET /api/v1/dashboard/vendor — métricas de vendedor.
 * Vendedor: vê apenas os próprios (JWT sobrescreve ?vendorId).
 * Gestor: ?vendorId obrigatório.
 */
export function useVendorDashboard(filter?: DashboardFilter) {
  return useQuery({
    queryKey: dashboardKeys.vendor(filter),
    queryFn: () =>
      fetchJSON(
        `/dashboard/vendor${buildDashboardQuery(filter)}`,
        VendorDashboardSchema,
      ),
  });
}

/**
 * GET /api/v1/dashboard/commissions/pending — indicadores de pendências (FR-019).
 * Apenas Gestor e Financeiro.
 */
export function usePendingCommissions() {
  return useQuery({
    queryKey: dashboardKeys.pending(),
    queryFn: () =>
      fetchJSON('/dashboard/commissions/pending', PendingCommissionsSchema),
  });
}

/**
 * GET /api/v1/orders/{id}/drilldown — rastreabilidade pedido → comissão → estorno (SC-004).
 * Vendedor: vê apenas os próprios; Gestor: qualquer.
 */
export function useDrillDown(orderId: string) {
  return useQuery({
    queryKey: dashboardKeys.drilldown(orderId),
    queryFn: () => fetchJSON(`/orders/${orderId}/drilldown`, DrillDownSchema),
    enabled: Boolean(orderId),
  });
}
