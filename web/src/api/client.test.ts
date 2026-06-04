// Testes Vitest para o API client
// Task 7.2.7: mock de fetch; zod.parse rejeita resposta malformada; token Bearer header
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { z } from 'zod';

import { ApiError, ValidationError, fetchJSON, getAccessToken, setAccessToken } from './client';

// ─── Helpers ──────────────────────────────────────────────────────────────────

function mockFetch(status: number, body: unknown): void {
  const response = new Response(
    body !== undefined ? JSON.stringify(body) : null,
    {
      status,
      headers: { 'Content-Type': 'application/json' },
    },
  );
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(response));
}

// ─── Setup / teardown ─────────────────────────────────────────────────────────

beforeEach(() => {
  setAccessToken(null);
});

afterEach(() => {
  vi.restoreAllMocks();
  setAccessToken(null);
});

// ─── Testes ───────────────────────────────────────────────────────────────────

describe('fetchJSON — token Bearer (CHK026)', () => {
  it('envia Authorization Bearer quando access_token está definido', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ id: '123', name: 'Test' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    );
    vi.stubGlobal('fetch', fetchMock);

    setAccessToken('token-abc-123');

    const schema = z.object({ id: z.string(), name: z.string() });
    await fetchJSON('/test', schema);

    const [_url, options] = fetchMock.mock.calls[0] as [string, RequestInit];
    const headers = options.headers as Record<string, string>;
    expect(headers['Authorization']).toBe('Bearer token-abc-123');
  });

  it('NÃO envia Authorization quando sem token', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ id: '1' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    );
    vi.stubGlobal('fetch', fetchMock);

    setAccessToken(null);

    const schema = z.object({ id: z.string() });
    await fetchJSON('/test', schema);

    const [_url, options] = fetchMock.mock.calls[0] as [string, RequestInit];
    const headers = options.headers as Record<string, string>;
    expect(headers['Authorization']).toBeUndefined();
  });
});

describe('fetchJSON — validação Zod (CHK026)', () => {
  it('retorna dados validados quando resposta corresponde ao schema', async () => {
    mockFetch(200, { id: 'uuid-001', name: 'Vendedor A' });

    const schema = z.object({ id: z.string(), name: z.string() });
    const result = await fetchJSON('/test', schema);

    expect(result).toEqual({ id: 'uuid-001', name: 'Vendedor A' });
  });

  it('lança ValidationError quando resposta não corresponde ao schema', async () => {
    // Campo 'id' é obrigatório mas está faltando
    mockFetch(200, { nome: 'sem-id' });

    const schema = z.object({ id: z.string().uuid() });

    await expect(fetchJSON('/test', schema)).rejects.toBeInstanceOf(ValidationError);
  });

  it('lança ValidationError quando totalCents é float (P-III violação)', async () => {
    // P-III: totalCents deve ser inteiro; float deve ser rejeitado
    mockFetch(200, { totalCents: 123.45 });

    const schema = z.object({ totalCents: z.number().int() });

    await expect(fetchJSON('/test', schema)).rejects.toBeInstanceOf(ValidationError);
  });

  it('aceita totalCents inteiro (P-III cumprido)', async () => {
    mockFetch(200, { totalCents: 12345 });

    const schema = z.object({ totalCents: z.number().int() });
    const result = await fetchJSON('/test', schema);

    expect(result.totalCents).toBe(12345);
  });
});

describe('fetchJSON — erros HTTP', () => {
  it('lança ApiError com status 401', async () => {
    mockFetch(401, { error: 'unauthorized' });

    const schema = z.object({ id: z.string() });

    const err = await fetchJSON('/test', schema).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).status).toBe(401);
  });

  it('lança ApiError com status 403', async () => {
    mockFetch(403, { error: 'forbidden' });

    const schema = z.object({ id: z.string() });

    const err = await fetchJSON('/test', schema).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).status).toBe(403);
  });

  it('lança ApiError com status 500', async () => {
    mockFetch(500, { error: 'internal_error' });

    const schema = z.object({ id: z.string() });

    const err = await fetchJSON('/test', schema).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).status).toBe(500);
  });
});

describe('setAccessToken / getAccessToken', () => {
  it('retorna null antes de definir token', () => {
    expect(getAccessToken()).toBeNull();
  });

  it('armazena e recupera token corretamente', () => {
    setAccessToken('meu-token');
    expect(getAccessToken()).toBe('meu-token');
  });

  it('limpa token ao chamar setAccessToken(null)', () => {
    setAccessToken('meu-token');
    setAccessToken(null);
    expect(getAccessToken()).toBeNull();
  });
});
