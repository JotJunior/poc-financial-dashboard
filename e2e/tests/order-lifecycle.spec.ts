/**
 * E2E: order-lifecycle.spec.ts — US2 Independent Test
 * Task 9.2.2: criar pedido, transicionar rascunho→confirmado→pago, verificar trilha.
 * Ref: tasks.md §9.2.2; quickstart.md §US2
 * API usa camelCase: vendorId, totalCents, orderDate, paidAt
 */
import { test, expect } from '../fixtures/auth';

const API_BASE = process.env.API_BASE_URL || 'http://localhost:8080/api/v1';
const E2E_VENDOR_ID = 'e2e00001-0000-0000-0000-000000000001';

test.describe('US2 — Order Lifecycle', () => {
  test('criar pedido em status rascunho', async ({ request, gestorToken }) => {
    const authHeader = `Bearer ${gestorToken}`;

    const createResp = await request.post(`${API_BASE}/orders`, {
      headers: { Authorization: authHeader },
      data: {
        vendorId: E2E_VENDOR_ID,
        totalCents: 150000,
        orderDate: '2026-11-01',
        items: [
          { description: 'Produto E2E', quantity: 3, unitPriceCents: 50000, lineTotalCents: 150000 }
        ],
      },
    });
    expect(createResp.status(), `Create order: ${await createResp.text()}`).toBe(201);
    const order = await createResp.json();
    expect(order).toHaveProperty('id');
    expect(order.status).toBe('rascunho');
    expect(order.totalCents).toBe(150000);
  });

  test('ciclo completo: rascunho → confirmado → pago + paid_at preenchido', async ({ request, gestorToken }) => {
    const authHeader = `Bearer ${gestorToken}`;

    // Criar pedido
    const createResp = await request.post(`${API_BASE}/orders`, {
      headers: { Authorization: authHeader },
      data: {
        vendorId: E2E_VENDOR_ID,
        totalCents: 200000,
        orderDate: '2026-11-05',
        items: [
          { description: 'Item E2E Ciclo', quantity: 2, unitPriceCents: 100000, lineTotalCents: 200000 }
        ],
      },
    });
    expect(createResp.status(), `Create: ${await createResp.text()}`).toBe(201);
    const order = await createResp.json();
    const orderID = order.id;

    // rascunho → confirmado
    const confirmResp = await request.patch(`${API_BASE}/orders/${orderID}/status`, {
      headers: { Authorization: authHeader },
      data: { status: 'confirmado' },
    });
    expect(confirmResp.status(), `Confirm: ${await confirmResp.text()}`).toBe(200);
    expect((await confirmResp.json()).status).toBe('confirmado');

    // confirmado → pago
    const payResp = await request.patch(`${API_BASE}/orders/${orderID}/status`, {
      headers: { Authorization: authHeader },
      data: { status: 'pago' },
    });
    expect(payResp.status(), `Pay: ${await payResp.text()}`).toBe(200);
    const paid = await payResp.json();
    expect(paid.status).toBe('pago');
    expect(paid.paidAt).not.toBeNull();
    expect(paid.paidAt).not.toBeUndefined();

    // Buscar e verificar estado final
    const getResp = await request.get(`${API_BASE}/orders/${orderID}`, {
      headers: { Authorization: authHeader },
    });
    expect(getResp.status()).toBe(200);
    expect((await getResp.json()).status).toBe('pago');
  });

  test('transição inválida retorna 400 (rascunho → pago direto)', async ({ request, gestorToken }) => {
    const authHeader = `Bearer ${gestorToken}`;

    const createResp = await request.post(`${API_BASE}/orders`, {
      headers: { Authorization: authHeader },
      data: { vendorId: E2E_VENDOR_ID, totalCents: 50000, orderDate: '2026-11-10' },
    });
    expect(createResp.status(), `Create: ${await createResp.text()}`).toBe(201);
    const order = await createResp.json();

    // rascunho → pago direto = inválido
    const invalidResp = await request.patch(`${API_BASE}/orders/${order.id}/status`, {
      headers: { Authorization: authHeader },
      data: { status: 'pago' },
    });
    expect([400, 422]).toContain(invalidResp.status());
  });

  test('pedido sem autenticação retorna 401', async ({ request }) => {
    const resp = await request.get(`${API_BASE}/orders`);
    expect(resp.status()).toBe(401);
  });
});
