/**
 * E2E: rbac.spec.ts — SC-005
 * Task 9.2.6: Vendedor não acessa dados de outros; Financeiro não acessa cadastro de vendedores.
 * Ref: tasks.md §9.2.6; quickstart.md §RBAC
 */
import { test, expect } from '../fixtures/auth';

const API_BASE = process.env.API_BASE_URL || 'http://localhost:8080/api/v1';
const E2E_VENDOR_ID_1 = 'e2e00001-0000-0000-0000-000000000001';
const E2E_VENDOR_ID_2 = 'e2e00001-0000-0000-0000-000000000002';

test.describe('SC-005 — RBAC Cross-Vendor Isolation', () => {
  test('Vendedor não acessa /dashboard/consolidated (apenas Gestor/Financeiro)', async ({ request, vendedorToken }) => {
    const resp = await request.get(`${API_BASE}/dashboard/consolidated`, {
      headers: { Authorization: `Bearer ${vendedorToken}` },
    });
    expect(resp.status()).toBe(403);
  });

  test('Vendedor não acessa dados de outro vendedor via GET /vendors/{id} (SC-005)', async ({ request, vendedorToken }) => {
    const resp = await request.get(`${API_BASE}/vendors/${E2E_VENDOR_ID_2}`, {
      headers: { Authorization: `Bearer ${vendedorToken}` },
    });
    expect(resp.status()).toBe(403);
  });

  test('Vendedor acessa apenas seu próprio dashboard via /dashboard/vendor', async ({ request, vendedorToken }) => {
    const resp = await request.get(`${API_BASE}/dashboard/vendor`, {
      headers: { Authorization: `Bearer ${vendedorToken}` },
    });
    expect(resp.status(), `Vendor dashboard: ${await resp.text()}`).toBe(200);
    const data = await resp.json();
    expect(typeof data.totalSalesCents).toBe('number');
  });

  test('Vendedor não lista comissões de outro vendedor (SC-005)', async ({ request, vendedorToken }) => {
    const resp = await request.get(`${API_BASE}/commissions?vendor_id=${E2E_VENDOR_ID_2}`, {
      headers: { Authorization: `Bearer ${vendedorToken}` },
    });
    if (resp.status() === 200) {
      const comms = await resp.json();
      // Nenhuma comissão deve pertencer ao vendor2
      const vendor2Comms = comms.filter((c: any) =>
        c.vendor_id === E2E_VENDOR_ID_2 || c.vendorId === E2E_VENDOR_ID_2
      );
      expect(vendor2Comms.length).toBe(0);
    } else {
      expect(resp.status()).toBe(403);
    }
  });

  test('Financeiro NÃO pode criar vendedor (RBAC)', async ({ request, financeiroToken }) => {
    const resp = await request.post(`${API_BASE}/vendors`, {
      headers: { Authorization: `Bearer ${financeiroToken}` },
      data: {
        name: 'Tentativa Financeiro',
        email: `fin-proibido-${Date.now()}@e2e.test`,
        commissionPercentage: '5.0000',
      },
    });
    expect(resp.status()).toBe(403);
  });

  test('Financeiro NÃO pode deletar/anonimizar vendedor', async ({ request, financeiroToken }) => {
    const resp = await request.delete(`${API_BASE}/vendors/${E2E_VENDOR_ID_2}`, {
      headers: { Authorization: `Bearer ${financeiroToken}` },
      data: { confirm: true },
    });
    expect(resp.status()).toBe(403);
  });

  test('Vendedor NÃO pode criar pedidos (apenas Gestor)', async ({ request, vendedorToken }) => {
    const resp = await request.post(`${API_BASE}/orders`, {
      headers: { Authorization: `Bearer ${vendedorToken}` },
      data: { vendorId: E2E_VENDOR_ID_1, totalCents: 10000, orderDate: '2026-06-01' },
    });
    expect(resp.status()).toBe(403);
  });

  test('Vendedor NÃO pode transicionar pedidos (apenas Gestor)', async ({ request, gestorToken, vendedorToken }) => {
    // Criar pedido como gestor
    const createResp = await request.post(`${API_BASE}/orders`, {
      headers: { Authorization: `Bearer ${gestorToken}` },
      data: { vendorId: E2E_VENDOR_ID_1, totalCents: 20000, orderDate: '2026-12-15' },
    });
    expect(createResp.status(), `Create: ${await createResp.text()}`).toBe(201);
    const orderID = (await createResp.json()).id;

    // Vendedor tenta transicionar → 403
    const transResp = await request.patch(`${API_BASE}/orders/${orderID}/status`, {
      headers: { Authorization: `Bearer ${vendedorToken}` },
      data: { status: 'confirmado' },
    });
    expect(transResp.status()).toBe(403);
  });

  test('sem autenticação: todos os endpoints retornam 401', async ({ request }) => {
    const endpoints = [
      { method: 'GET', path: '/vendors' },
      { method: 'GET', path: '/orders' },
      { method: 'GET', path: '/commissions' },
      { method: 'GET', path: '/dashboard/consolidated' },
      { method: 'GET', path: '/dashboard/vendor' },
    ];

    for (const ep of endpoints) {
      const resp = await request.fetch(`${API_BASE}${ep.path}`, { method: ep.method });
      expect(resp.status(), `${ep.method} ${ep.path}`).toBe(401);
    }
  });
});
