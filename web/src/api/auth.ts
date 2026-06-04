// API hooks para Autenticação — react-query + fetchJSON + Zod
// Task 7.2.6: useLogin, useLogout, useRefreshToken
// CHK026: access_token em memória; refresh_token NUNCA tocado em JS (httpOnly cookie)
// Ref: contracts/api.md §/auth; auth.ts; auth.schema.ts
import { useMutation, useQueryClient } from '@tanstack/react-query';

import { fetchJSON, setAccessToken } from './client';
import type { LoginRequest } from '../types/auth';
import { LoginResponseSchema } from '../types/auth.schema';
import { z } from 'zod';

// ─── Mutations ────────────────────────────────────────────────────────────────

/**
 * POST /api/v1/auth/login — autentica e armazena access_token em memória.
 * refresh_token é retornado pelo backend em httpOnly cookie — nunca manipulado aqui.
 */
export function useLogin() {
  return useMutation({
    mutationFn: async (req: LoginRequest) => {
      const data = await fetchJSON('/auth/login', LoginResponseSchema, {
        method: 'POST',
        body: req,
      });
      // Armazenar em memória — NUNCA localStorage (CHK026)
      setAccessToken(data.accessToken);
      return data;
    },
  });
}

/**
 * POST /api/v1/auth/logout — invalida o token no backend e limpa o estado local.
 */
export function useLogout() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      // Schema vazio — logout retorna 204 No Content
      await fetchJSON('/auth/logout', z.unknown(), { method: 'POST' });
      setAccessToken(null);
    },
    onSuccess: () => {
      // Limpar todo o cache react-query ao deslogar
      qc.clear();
    },
  });
}

/**
 * POST /api/v1/auth/refresh — renova o access_token usando o refresh_token httpOnly cookie.
 * Chamado automaticamente quando fetchJSON retornar 401.
 * CHK026: nunca lê o refresh_token no JS — o browser envia via cookie automaticamente.
 */
export async function refreshAccessToken(): Promise<string | null> {
  try {
    const data = await fetchJSON('/auth/refresh', LoginResponseSchema, {
      method: 'POST',
    });
    setAccessToken(data.accessToken);
    return data.accessToken;
  } catch {
    // Refresh falhou — sessão expirada, limpar token
    setAccessToken(null);
    return null;
  }
}

/**
 * Hook para renovação manual do token (raro — prefira o interceptor automático).
 */
export function useRefreshToken() {
  return useMutation({
    mutationFn: refreshAccessToken,
  });
}
