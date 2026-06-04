// Schemas Zod para Auth — parse em toda resposta da API (borda de validação)
// Task 7.1.5: paridade exata com contracts/api.md §/auth e auth.ts
import { z } from 'zod';

export const UserRoleSchema = z.enum(['gestor', 'vendedor', 'financeiro']);

export const LoginRequestSchema = z.object({
  email: z.string().email(),
  password: z.string().min(1),
});

// Backend retorna snake_case: {"access_token":"..."} (contracts/api.md §/auth)
// Transformar para camelCase internamente para consistência com o restante do código.
export const LoginResponseSchema = z.object({
  access_token: z.string().min(1),
  // refresh_token: httpOnly cookie — não aparece no JSON (CHK026)
}).transform(raw => ({ accessToken: raw.access_token }));

export const AuthUserSchema = z.object({
  sub: z.string().uuid(),
  role: UserRoleSchema,
  vendorId: z.string().uuid().nullable(),
  exp: z.number().int(),
});

export type UserRoleT = z.infer<typeof UserRoleSchema>;
export type LoginResponseT = z.infer<typeof LoginResponseSchema>;
export type AuthUserT = z.infer<typeof AuthUserSchema>;
