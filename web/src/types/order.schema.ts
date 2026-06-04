// Schemas Zod para Order — parse em toda resposta da API (borda de validação)
// Task 7.1.5: paridade exata com contracts/api.md §/orders e order.ts
// P-III: totalCents é number inteiro — zod.int() rejeita float
import { z } from 'zod';

export const OrderStatusSchema = z.enum(['rascunho', 'confirmado', 'pago', 'cancelado']);

export const OrderItemSchema = z.object({
  id: z.string().uuid(),
  description: z.string().max(500),
  quantity: z.number().int().positive(),
  unitPriceCents: z.number().int().nonnegative(), // P-III: inteiro centavos
  lineTotalCents: z.number().int().nonnegative(), // P-III: inteiro centavos
});

export const OrderSchema = z.object({
  id: z.string().uuid(),
  vendorId: z.string().uuid(),
  totalCents: z.number().int().nonnegative(), // P-III: NUNCA float
  orderDate: z.string().regex(/^\d{4}-\d{2}-\d{2}$/),
  status: OrderStatusSchema,
  paidAt: z.string().nullable(),
  items: z.array(OrderItemSchema),
});

export const OrderListSchema = z.array(OrderSchema);

export const CreateOrderItemRequestSchema = z.object({
  description: z.string().min(1).max(500),
  quantity: z.number().int().positive(),
  unitPriceCents: z.number().int().nonnegative(),
});

export const CreateOrderRequestSchema = z.object({
  vendorId: z.string().uuid(),
  orderDate: z.string().regex(/^\d{4}-\d{2}-\d{2}$/),
  items: z.array(CreateOrderItemRequestSchema).min(1),
});

export const TransitionOrderRequestSchema = z.object({
  status: OrderStatusSchema,
});

export type OrderStatusT = z.infer<typeof OrderStatusSchema>;
export type OrderItemT = z.infer<typeof OrderItemSchema>;
export type OrderT = z.infer<typeof OrderSchema>;
