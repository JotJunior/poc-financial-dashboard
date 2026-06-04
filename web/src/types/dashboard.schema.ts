// Schemas Zod para Dashboard — parse em toda resposta da API (borda de validação)
// Task 7.1.5: paridade exata com contracts/api.md §6 Dashboards e dashboard.ts
// P-III: todos os campos *Cents são number inteiro — nunca float
import { z } from 'zod';

// ─── Consolidated Dashboard ───────────────────────────────────────────────────

export const VendorRankingSchema = z.object({
  vendorId: z.string().min(1),
  vendorName: z.string(),
  totalCents: z.number().int().nonnegative(),  // P-III
  orderCount: z.number().int().nonnegative(),
  commCents: z.number().int().nonnegative(),   // P-III
});

export const ConsolidatedDashboardSchema = z.object({
  totalSalesCents: z.number().int().nonnegative(),   // P-III
  totalCommCents: z.number().int(),                  // P-III (pode ser 0)
  pendingCommCents: z.number().int().nonnegative(),  // P-III
  approvedCommCents: z.number().int().nonnegative(), // P-III
  paidCommCents: z.number().int().nonnegative(),     // P-III
  orderCount: z.number().int().nonnegative(),
  vendorCount: z.number().int().nonnegative(),
  topVendors: z.array(VendorRankingSchema),
});

// ─── Vendor Dashboard ─────────────────────────────────────────────────────────

export const VendorDashboardSchema = z.object({
  vendorId: z.string().min(1),
  totalSalesCents: z.number().int().nonnegative(),   // P-III
  totalCommCents: z.number().int(),                  // P-III
  pendingCommCents: z.number().int().nonnegative(),  // P-III
  approvedCommCents: z.number().int().nonnegative(), // P-III
  paidCommCents: z.number().int().nonnegative(),     // P-III
  orderCount: z.number().int().nonnegative(),
});

// ─── Pending Commissions ──────────────────────────────────────────────────────

export const PendingCommissionsSchema = z.object({
  pendingApprovalCount: z.number().int().nonnegative(),
  pendingApprovalCents: z.number().int().nonnegative(), // P-III
  approvedUnpaidCount: z.number().int().nonnegative(),
  approvedUnpaidCents: z.number().int().nonnegative(),  // P-III
});

// ─── DrillDown ───────────────────────────────────────────────────────────────

export const DrillDownReversalSchema = z.object({
  reversalId: z.string().min(1),
  valueCents: z.number().int().max(0), // negativo (P-III)
  status: z.string(),
  createdAt: z.string(),
  actorUserId: z.string().min(1), // auditabilidade P-I
});

export const DrillDownCommissionSchema = z.object({
  commissionId: z.string().min(1),
  valueCents: z.number().int().nonnegative(), // P-III
  netCents: z.number().int(),                 // P-III (pode ser negativo)
  status: z.string(),
  periodYear: z.number().int(),
  periodMonth: z.number().int().min(1).max(12),
  reversals: z.array(DrillDownReversalSchema),
});

export const DrillDownSchema = z.object({
  orderId: z.string().min(1),
  vendorId: z.string().min(1),
  totalCents: z.number().int().nonnegative(), // P-III
  orderDate: z.string().regex(/^\d{4}-\d{2}-\d{2}$/),
  status: z.string(),
  createdAt: z.string(),
  commissions: z.array(DrillDownCommissionSchema),
});

export type ConsolidatedDashboardT = z.infer<typeof ConsolidatedDashboardSchema>;
export type VendorDashboardT = z.infer<typeof VendorDashboardSchema>;
export type PendingCommissionsT = z.infer<typeof PendingCommissionsSchema>;
export type DrillDownT = z.infer<typeof DrillDownSchema>;
