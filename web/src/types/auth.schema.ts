// Schemas Zod para Auth — parse em toda resposta da API (borda de validação)
// Task 7.1.5: paridade exata com contracts/api.md §/auth e auth.ts
import { z } from 'zod';

export const UserRoleSchema = z.enum(['gestor', 'vendedor', 'financeiro']);

export const LoginRequestSchema = z.object({
  email: z.string().email(),
  password: z.string().min(1),
});

export const LoginResponseSchema = z.object({
  accessToken: z.string().min(1),
  // refresh_token: httpOnly cookie — não aparece no JSON (CHK026)
});

export const AuthUserSchema = z.object({
  sub: z.string().uuid(),
  role: UserRoleSchema,
  vendorId: z.string().uuid().nullable(),
  exp: z.number().int(),
});

export type UserRoleT = z.infer<typeof UserRoleSchema>;
export type LoginResponseT = z.infer<typeof LoginResponseSchema>;
export type AuthUserT = z.infer<typeof AuthUserSchema>;
