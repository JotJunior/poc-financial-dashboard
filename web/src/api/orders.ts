// API hooks para Pedidos — react-query + fetchJSON + Zod
// Task 7.2.3: useOrders, useOrder, useCreateOrder, useTransitionOrder
// Ref: contracts/api.md §/orders; order.ts; order.schema.ts
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { fetchJSON } from './client';
import type { Order, OrderStatus } from '../types/order';
import { OrderListSchema, OrderSchema } from '../types/order.schema';

// ─── Filter type ──────────────────────────────────────────────────────────────

export interface OrderFilter {
  vendorId?: string;
  status?: OrderStatus;
  year?: number;
  month?: number;
}

function buildOrderQuery(filter?: OrderFilter): string {
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

export const orderKeys = {
  all: ['orders'] as const,
  list: (filter?: OrderFilter) => [...orderKeys.all, 'list', filter ?? {}] as const,
  detail: (id: string) => [...orderKeys.all, 'detail', id] as const,
};

// ─── Queries ──────────────────────────────────────────────────────────────────

/** GET /api/v1/orders — lista pedidos (Vendedor com escopo, Gestor todos) */
export function useOrders(filter?: OrderFilter) {
  return useQuery({
    queryKey: orderKeys.list(filter),
    queryFn: () => fetchJSON(`/orders${buildOrderQuery(filter)}`, OrderListSchema),
  });
}

/** GET /api/v1/orders/{id} — detalhe de um pedido */
export function useOrder(id: string) {
  return useQuery({
    queryKey: orderKeys.detail(id),
    queryFn: () => fetchJSON(`/orders/${id}`, OrderSchema),
    enabled: Boolean(id),
  });
}

// ─── Mutations ────────────────────────────────────────────────────────────────

export interface CreateOrderItem {
  description: string;
  quantity: number;
  unitPriceCents: number; // P-III: centavos inteiros
}

export interface CreateOrderRequest {
  vendorId: string;
  orderDate: string; // "YYYY-MM-DD"
  items: CreateOrderItem[];
}

/** POST /api/v1/orders — criar pedido (Gestor) */
export function useCreateOrder() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: CreateOrderRequest) =>
      fetchJSON('/orders', OrderSchema, { method: 'POST', body: req }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: orderKeys.all });
    },
  });
}

/** PATCH /api/v1/orders/{id}/status — transicionar estado (Gestor) */
export function useTransitionOrder() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, status }: { id: string; status: OrderStatus }) =>
      fetchJSON(`/orders/${id}/status`, OrderSchema, {
        method: 'PATCH',
        body: { status },
      }),
    onSuccess: (updated: Order) => {
      qc.setQueryData(orderKeys.detail(updated.id), updated);
      void qc.invalidateQueries({ queryKey: orderKeys.all });
    },
  });
}
