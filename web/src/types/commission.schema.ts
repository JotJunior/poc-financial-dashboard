// Schemas Zod para Commission — parse em toda resposta da API (borda de validação)
// Task 7.1.5: paridade exata com contracts/api.md §/commissions e commission.ts
// P-III: valueCents/netCents são number inteiro — nunca float
import { z } from 'zod';

export const CommissionStatusSchema = z.enum(['pendente', 'aprovado', 'pago']);
export const ReversalStatusSchema = z.enum(['aplicado', 'pendente_aprovacao', 'aprovado', 'lancado']);

export const CommissionSchema = z.object({
  id: z.string().min(1),
  orderId: z.string().min(1),
  vendorId: z.string().min(1),
  valueCents: z.number().int().nonnegative(),   // P-III: NUNCA float
  netCents: z.number().int(),                   // P-III: pode ser negativo com estornos
  appliedPercentage: z.string().regex(/^\d+\.\d{4}$/), // "5.5000"
  ruleId: z.string().min(1),
  periodYear: z.number().int().min(2020).max(2100),
  periodMonth: z.number().int().min(1).max(12),
  status: CommissionStatusSchema,
  calculatedAt: z.string(), // ISO 8601 UTC
});

export const CommissionListSchema = z.array(CommissionSchema);

export const CommissionReversalSchema = z.object({
  id: z.string().min(1),
  commissionId: z.string().min(1),
  orderId: z.string().min(1),
  valueCents: z.number().int().max(0), // negativo ou zero (P-III)
  status: ReversalStatusSchema,
  createdAt: z.string(),
  actorUserId: z.string().min(1),
});

export const CommissionNetBalanceSchema = z.object({
  commissionId: z.string().min(1),
  vendorId: z.string().min(1),
  netCents: z.number().int(), // pode ser negativo (P-III)
});

export const ApurationRequestSchema = z.object({
  year: z.number().int().min(2020).max(2100),
  month: z.number().int().min(1).max(12),
});

export const ApurationResultSchema = z.object({
  periodYear: z.number().int(),
  periodMonth: z.number().int().min(1).max(12),
  calculated: z.number().int().nonnegative(),
  skipped: z.number().int().nonnegative(),
  totalCents: z.number().int().nonnegative(), // P-III: NUNCA float
});

export const TransitionCommissionRequestSchema = z.object({
  status: CommissionStatusSchema,
  motivo: z.string().optional(),
});

export type CommissionStatusT = z.infer<typeof CommissionStatusSchema>;
export type CommissionT = z.infer<typeof CommissionSchema>;
export type ApurationResultT = z.infer<typeof ApurationResultSchema>;
