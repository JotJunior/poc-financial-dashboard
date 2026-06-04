// API client base — fetchJSON com autenticação Bearer e validação Zod
// Task 7.2.1: fetch + status check + zod.parse na resposta
// CHK026: access_token enviado como Bearer header; refresh_token NUNCA tocado em JS (httpOnly cookie)
// Ref: contracts/api.md §autenticação; spec §FR-024-026
import { z } from 'zod';

// ─── Configuração ─────────────────────────────────────────────────────────────

const API_BASE = import.meta.env.VITE_API_BASE_URL ?? '/api/v1';

// Token store em memória — NUNCA localStorage/sessionStorage (CHK026)
let _accessToken: string | null = null;

export function setAccessToken(token: string | null): void {
  _accessToken = token;
}

export function getAccessToken(): string | null {
  return _accessToken;
}

// ─── Erros tipados ────────────────────────────────────────────────────────────

export class ApiError extends Error {
  readonly status: number;
  readonly body: unknown;
  constructor(
    status: number,
    body: unknown,
    message?: string,
  ) {
    super(message ?? `API error ${status}`);
    this.name = 'ApiError';
    this.status = status;
    this.body = body;
  }
}

export class ValidationError extends Error {
  readonly issues: z.ZodIssue[];
  constructor(
    issues: z.ZodIssue[],
    message?: string,
  ) {
    super(message ?? `Resposta inválida da API: ${issues.map((i) => i.message).join('; ')}`);
    this.name = 'ValidationError';
    this.issues = issues;
  }
}

// ─── fetchJSON<T> ─────────────────────────────────────────────────────────────

export interface FetchOptions extends Omit<RequestInit, 'body'> {
  body?: unknown;
}

/**
 * Faz fetch para a API, verifica o status HTTP, e valida a resposta com um schema Zod.
 * CHK026: envia o accessToken como Bearer header quando disponível.
 * Nunca lê/escreve refresh_token no JS — ele fica em httpOnly cookie.
 */
export async function fetchJSON<T>(
  path: string,
  schema: z.ZodType<T>,
  options: FetchOptions = {},
): Promise<T> {
  const { body, headers: extraHeaders, ...rest } = options;

  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(extraHeaders as Record<string, string> | undefined),
  };

  // Injetar Bearer token quando disponível (CHK026)
  if (_accessToken) {
    headers['Authorization'] = `Bearer ${_accessToken}`;
  }

  const response = await fetch(`${API_BASE}${path}`, {
    ...rest,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
    // Habilitar envio de cookies (refresh_token httpOnly) em todas as requests
    credentials: 'include',
  });

  // Respostas 204 No Content não têm body
  if (response.status === 204) {
    return undefined as T;
  }

  let rawJson: unknown;
  try {
    rawJson = await response.json();
  } catch {
    throw new ApiError(response.status, null, `Resposta não-JSON (status ${response.status})`);
  }

  if (!response.ok) {
    throw new ApiError(response.status, rawJson);
  }

  // Validação Zod na borda — parse na resposta antes de retornar (CHK026)
  const result = schema.safeParse(rawJson);
  if (!result.success) {
    throw new ValidationError(result.error.issues);
  }

  return result.data;
}
