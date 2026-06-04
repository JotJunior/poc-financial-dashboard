/**
 * E2E: dashboard.spec.ts — SC-004, SC-009
 * Task 9.2.5: dashboard carrega < 3s (SC-009); drill-down em <= 3 cliques (SC-004).
 * Ref: tasks.md §9.2.5; quickstart.md §Dashboard
 * API usa camelCase: totalSalesCents, totalCommCents, orderCount
 */
import { test, expect } from '../fixtures/auth';

const API_BASE = process.env.API_BASE_URL || 'http://localhost:8080/api/v1';
const E2E_VENDOR_ID = 'e2e00001-0000-0000-0000-000000000001';

test.describe('Dashboard — SC-004, SC-009', () => {
  test('GET /dashboard/consolidated retorna 200 em tempo < 3s (SC-009)', async ({ request, gestorToken }) => {
    const start = Date.now();
    const resp = await request.get(`${API_BASE}/dashboard/consolidated`, {
      headers: { Authorization: `Bearer ${gestorToken}` },
    });
    const elapsed = Date.now() - start;

    expect(resp.status(), `Dashboard: ${await resp.text()}`).toBe(200);
    const dashboard = await resp.json();
    expect(typeof dashboard.totalSalesCents).toBe('number');
    expect(typeof dashboard.orderCount).toBe('number');

    // SC-009: dashboard deve carregar em < 3000ms
    expect(elapsed).toBeLessThan(3000);
  });

  test('GET /dashboard/vendor retorna dados do vendedor autenticado', async ({ request, vendedorToken }) => {
    const resp = await request.get(`${API_BASE}/dashboard/vendor`, {
      headers: { Authorization: `Bearer ${vendedorToken}` },
    });
    expect(resp.status(), `Vendor dashboard: ${await resp.text()}`).toBe(200);
    const dashboard = await resp.json();
    expect(typeof dashboard.totalSalesCents).toBe('number');
    expect(typeof dashboard.totalCommCents).toBe('number');
  });

  test('Financeiro pode acessar dashboard consolidado', async ({ request, financeiroToken }) => {
    const resp = await request.get(`${API_BASE}/dashboard/consolidated`, {
      headers: { Authorization: `Bearer ${financeiroToken}` },
    });
    expect(resp.status(), `Fin dashboard: ${await resp.text()}`).toBe(200);
    const data = await resp.json();
    expect(typeof data.totalSalesCents).toBe('number');
  });

  test('drill-down via /orders/{id}/drilldown (SC-004: max 3 requests)', async ({ request, gestorToken }) => {
    const authHeader = `Bearer ${gestorToken}`;

    // Request 1: dashboard consolidado
    await request.get(`${API_BASE}/dashboard/consolidated`, { headers: { Authorization: authHeader } });

    // Request 2: listar pedidos
    const ordersResp = await request.get(`${API_BASE}/orders`, { headers: { Authorization: authHeader } });
    expect(ordersResp.status()).toBe(200);
    const ordersBody = await ordersResp.json();
    const orders = Array.isArray(ordersBody) ? ordersBody : (ordersBody.orders || []);

    if (orders.length === 0) return;

    // Request 3: drill-down no primeiro pedido (SC-004: total 3 requests — satisfeito)
    const orderID = orders[0].id;
    const drillResp = await request.get(`${API_BASE}/orders/${orderID}/drilldown`, {
      headers: { Authorization: authHeader },
    });
    expect(drillResp.status(), `Drilldown: ${await drillResp.text()}`).toBe(200);
    const drillData = await drillResp.json();
    // Drilldown retorna dados do pedido diretamente (orderId, vendorId, commissions, etc.)
    expect(drillData).toHaveProperty('orderId');
  });

  test('dashboard consolidado inacessível para vendedor (RBAC)', async ({ request, vendedorToken }) => {
    const resp = await request.get(`${API_BASE}/dashboard/consolidated`, {
      headers: { Authorization: `Bearer ${vendedorToken}` },
    });
    expect(resp.status()).toBe(403);
  });

  test('GET /dashboard/commissions/pending retorna sumário pendente (Gestor)', async ({ request, gestorToken }) => {
    const resp = await request.get(`${API_BASE}/dashboard/commissions/pending`, {
      headers: { Authorization: `Bearer ${gestorToken}` },
    });
    expect(resp.status(), `Pending: ${await resp.text()}`).toBe(200);
    const data = await resp.json();
    // Retorna objeto com campos de sumário
    expect(typeof data.pendingApprovalCount).toBe('number');
    expect(typeof data.pendingApprovalCents).toBe('number');
  });
});
