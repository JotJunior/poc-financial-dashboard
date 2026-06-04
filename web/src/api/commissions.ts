// API hooks para Comissões e Apuração — react-query + fetchJSON + Zod
// Task 7.2.4: useCommissions, useApurate, useTransitionCommission
// Ref: contracts/api.md §/commissions; commission.ts; commission.schema.ts
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { fetchJSON } from './client';
import type { ApurationRequest, Commission, CommissionStatus } from '../types/commission';
import {
  ApurationResultSchema,
  CommissionListSchema,
  CommissionSchema,
} from '../types/commission.schema';

// ─── Filter type ──────────────────────────────────────────────────────────────

export interface CommissionFilter {
  vendorId?: string;
  status?: CommissionStatus;
  year?: number;
  month?: number;
}

function buildCommissionQuery(filter?: CommissionFilter): string {
  if (!filter) return '';
  const params = new URLSearchParams();
  if (filter.vendorId) params.set('vendorId', filter.vendorId);
  if (filter.status) params.set('status', filter.status);
  if (filter.year) params.set('year', String(filter.year));
  if (filter.month) params.set('month', String(filter.month));
  const q = params.toString();
  return q ? `?${q}` : '';
}

// ─── Query keys ───────────────────────────────────────────────────────────────

export const commissionKeys = {
  all: ['commissions'] as const,
  list: (filter?: CommissionFilter) => [...commissionKeys.all, 'list', filter ?? {}] as const,
  detail: (id: string) => [...commissionKeys.all, 'detail', id] as const,
};

// ─── Queries ──────────────────────────────────────────────────────────────────

/** GET /api/v1/commissions — lista comissões (Vendedor com escopo) */
export function useCommissions(filter?: CommissionFilter) {
  return useQuery({
    queryKey: commissionKeys.list(filter),
    queryFn: () =>
      fetchJSON(`/commissions${buildCommissionQuery(filter)}`, CommissionListSchema),
  });
}

/** GET /api/v1/commissions/{id} — detalhe de comissão */
export function useCommission(id: string) {
  return useQuery({
    queryKey: commissionKeys.detail(id),
    queryFn: () => fetchJSON(`/commissions/${id}`, CommissionSchema),
    enabled: Boolean(id),
  });
}

// ─── Mutations ────────────────────────────────────────────────────────────────

/** POST /api/v1/commissions/apurate — apurar comissões do mês (Gestor) */
export function useApurate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: ApurationRequest) =>
      fetchJSON('/commissions/apurate', ApurationResultSchema, {
        method: 'POST',
        body: req,
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: commissionKeys.all });
    },
  });
}

/** PATCH /api/v1/commissions/{id}/status — transicionar estado (Financeiro) */
export function useTransitionCommission() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      status,
      motivo,
    }: {
      id: string;
      status: CommissionStatus;
      motivo?: string;
    }) =>
      fetchJSON(`/commissions/${id}/status`, CommissionSchema, {
        method: 'PATCH',
        body: { status, ...(motivo ? { motivo } : {}) },
      }),
    onSuccess: (updated: Commission) => {
      qc.setQueryData(commissionKeys.detail(updated.id), updated);
      void qc.invalidateQueries({ queryKey: commissionKeys.all });
    },
  });
}
