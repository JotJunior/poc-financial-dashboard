// Schemas Zod para Vendor — parse em toda resposta da API (borda de validação)
// Task 7.1.5: paridade exata com contracts/api.md §/vendors e vendor.ts
// CHK026: validar na borda — zod.parse() antes de usar qualquer dado da API
import { z } from 'zod';

export const VendorStatusSchema = z.enum(['ativo', 'inativo']);

export const VendorSchema = z.object({
  id: z.string().min(1),
  name: z.string().max(200),
  email: z.string().email(),
  status: VendorStatusSchema,
  anonymizedAt: z.string().nullable().optional().default(null), // ISO 8601 UTC; ausente = null
});

export const VendorListSchema = z.array(VendorSchema);

export const CreateVendorRequestSchema = z.object({
  name: z.string().min(1).max(200),
  email: z.string().email().max(255),
});

export const VendorPatchSchema = z.object({
  name: z.string().min(1).max(200).optional(),
  email: z.string().email().max(255).optional(),
  status: VendorStatusSchema.optional(),
});

export const CommissionRuleSchema = z.object({
  id: z.string().min(1),
  vendorId: z.string().min(1),
  percentage: z.string().regex(/^\d+\.\d{4}$/), // "5.5000" format
  validFrom: z.string(),
  validTo: z.string().nullable(),
  version: z.number().int().positive(),
});

export const SetCommissionRuleRequestSchema = z.object({
  percentage: z.string().regex(/^\d+\.\d{4}$/, 'Formato inválido — use "5.5000"'),
  validFrom: z.string().regex(/^\d{4}-\d{2}-\d{2}$/, 'Formato inválido — use YYYY-MM-DD'),
});

export type VendorStatusT = z.infer<typeof VendorStatusSchema>;
export type VendorT = z.infer<typeof VendorSchema>;
export type CommissionRuleT = z.infer<typeof CommissionRuleSchema>;
