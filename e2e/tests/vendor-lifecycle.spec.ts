/**
 * E2E: vendor-lifecycle.spec.ts — US1 Independent Test
 * Task 9.2.1: criar vendedor, alterar percentual, desativar.
 * Ref: tasks.md §9.2.1; quickstart.md §US1
 *
 * API usa camelCase: commissionPercentage, vendorId
 * Usa fixture compartilhado para evitar rate limit no login.
 */
import { test, expect } from '../fixtures/auth';

const API_BASE = process.env.API_BASE_URL || 'http://localhost:8080/api/v1';

test.describe('US1 — Vendor Lifecycle', () => {
  test('criar vendedor com regra de comissão', async ({ request, gestorToken }) => {
    const authHeader = `Bearer ${gestorToken}`;

    const createResp = await request.post(`${API_BASE}/vendors`, {
      headers: { Authorization: authHeader },
      data: {
        name: 'Vendedor E2E Lifecycle',
        email: `lifecycle-vendor-${Date.now()}@e2e.test`,
        commissionPercentage: '9.5000',
      },
    });
    expect(createResp.status(), `Create vendor: ${await createResp.text()}`).toBe(201);
    const vendor = await createResp.json();
    expect(vendor).toHaveProperty('id');
    expect(vendor.name).toBe('Vendedor E2E Lifecycle');
    expect(vendor.status).toBe('ativo');

    // Buscar vendedor criado
    const getResp = await request.get(`${API_BASE}/vendors/${vendor.id}`, {
      headers: { Authorization: authHeader },
    });
    expect(getResp.status()).toBe(200);
    const fetched = await getResp.json();
    expect(fetched.id).toBe(vendor.id);
  });

  test('alterar percentual de comissão (nova versão de regra)', async ({ request, gestorToken }) => {
    const authHeader = `Bearer ${gestorToken}`;

    const createResp = await request.post(`${API_BASE}/vendors`, {
      headers: { Authorization: authHeader },
      data: {
        name: 'Vendedor E2E Percentual',
        email: `percentual-${Date.now()}@e2e.test`,
        commissionPercentage: '8.0000',
      },
    });
    expect(createResp.status(), `Create: ${await createResp.text()}`).toBe(201);
    const vendor = await createResp.json();
    const vendorID = vendor.id;

    // Alterar percentual
    const patchResp = await request.patch(`${API_BASE}/vendors/${vendorID}`, {
      headers: { Authorization: authHeader },
      data: { commissionPercentage: '12.5000' },
    });
    expect(patchResp.status(), `Patch: ${await patchResp.text()}`).toBe(200);
    const updated = await patchResp.json();
    expect(updated.id).toBe(vendorID);
    expect(updated.status).toBe('ativo');
  });

  test('desativar vendedor', async ({ request, gestorToken }) => {
    const authHeader = `Bearer ${gestorToken}`;

    const createResp = await request.post(`${API_BASE}/vendors`, {
      headers: { Authorization: authHeader },
      data: {
        name: 'Vendedor E2E Desativar',
        email: `desativar-${Date.now()}@e2e.test`,
        commissionPercentage: '7.0000',
      },
    });
    expect(createResp.status(), `Create: ${await createResp.text()}`).toBe(201);
    const vendor = await createResp.json();
    const vendorID = vendor.id;

    // Desativar via PATCH status
    const deactivateResp = await request.patch(`${API_BASE}/vendors/${vendorID}`, {
      headers: { Authorization: authHeader },
      data: { status: 'inativo' },
    });
    expect(deactivateResp.status(), `Deactivate: ${await deactivateResp.text()}`).toBe(200);
    const deactivated = await deactivateResp.json();
    expect(deactivated.status).toBe('inativo');
  });

  test('vendedor não pode criar ou listar todos os vendedores (RBAC)', async ({ request, vendedorToken }) => {
    const vendorHeader = `Bearer ${vendedorToken}`;

    const listResp = await request.get(`${API_BASE}/vendors`, {
      headers: { Authorization: vendorHeader },
    });
    expect(listResp.status()).toBe(403);

    const createResp = await request.post(`${API_BASE}/vendors`, {
      headers: { Authorization: vendorHeader },
      data: {
        name: 'Tentativa Proibida',
        email: 'proibido@e2e.test',
        commissionPercentage: '5.0000',
      },
    });
    expect(createResp.status()).toBe(403);
  });
});
