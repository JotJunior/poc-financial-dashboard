/**
 * E2E: payment-lifecycle.spec.ts — US4 Independent Test
 * Task 9.2.4: Financeiro aprova comissão, marca como paga, Vendedor não pode aprovar.
 * Ref: tasks.md §9.2.4; quickstart.md §US4
 *
 * Nota: comissões são criadas automaticamente quando o pedido é pago.
 * O endpoint /commissions/{id}/status é para transicionar aprovação/pagamento.
 */
import { test, expect } from '../fixtures/auth';

const API_BASE = process.env.API_BASE_URL || 'http://localhost:8080/api/v1';
const E2E_VENDOR_ID = 'e2e00001-0000-0000-0000-000000000001';

async function createPendingCommission(
  request: any,
  gestorToken: string,
): Promise<string> {
  const authHeader = `Bearer ${gestorToken}`;

  // Criar pedido e pagar — comissão é criada automaticamente como 'pendente'
  const create = await request.post(`${API_BASE}/orders`, {
    headers: { Authorization: authHeader },
    data: {
      vendorId: E2E_VENDOR_ID,
      totalCents: 80000,
      orderDate: '2026-11-15',
    },
  });
  expect(create.status(), `Create order: ${await create.text()}`).toBe(201);
  const orderID = (await create.json()).id;

  await request.patch(`${API_BASE}/orders/${orderID}/status`, {
    headers: { Authorization: authHeader }, data: { status: 'confirmado' },
  });
  const pay = await request.patch(`${API_BASE}/orders/${orderID}/status`, {
    headers: { Authorization: authHeader }, data: { status: 'pago' },
  });
  expect(pay.status(), `Pay: ${await pay.text()}`).toBe(200);

  // Buscar a comissão criada automaticamente para este pedido
  const list = await request.get(`${API_BASE}/commissions?vendorId=${E2E_VENDOR_ID}`, {
    headers: { Authorization: authHeader },
  });
  expect(list.status()).toBe(200);
  const comms = await list.json();
  const comm = comms.find((c: any) =>
    (c.orderId === orderID || c.order_id === orderID) && c.status === 'pendente'
  );
  expect(comm, `Comissão pendente para pedido ${orderID} deve existir`).toBeDefined();
  return comm.id;
}

test.describe('US4 — Payment Lifecycle (Commission Approval)', () => {
  test('Financeiro aprova comissão pendente (pendente → aprovado)', async ({ request, gestorToken, financeiroToken }) => {
    const commID = await createPendingCommission(request, gestorToken);

    const approveResp = await request.patch(`${API_BASE}/commissions/${commID}/status`, {
      headers: { Authorization: `Bearer ${financeiroToken}` },
      data: { status: 'aprovado', motivo: 'Aprovado pelo financeiro E2E' },
    });
    expect(approveResp.status(), `Approve: ${await approveResp.text()}`).toBe(200);
    expect((await approveResp.json()).status).toBe('aprovado');
  });

  test('Financeiro marca comissão como paga (aprovado → pago)', async ({ request, gestorToken, financeiroToken }) => {
    const finHeader = `Bearer ${financeiroToken}`;

    const commID = await createPendingCommission(request, gestorToken);

    // Aprovar primeiro
    await request.patch(`${API_BASE}/commissions/${commID}/status`, {
      headers: { Authorization: finHeader },
      data: { status: 'aprovado', motivo: 'Aprovado para pagamento' },
    });

    // Marcar como pago
    const payResp = await request.patch(`${API_BASE}/commissions/${commID}/status`, {
      headers: { Authorization: finHeader },
      data: { status: 'pago', motivo: 'Pagamento efetuado via TED' },
    });
    expect(payResp.status(), `Pay: ${await payResp.text()}`).toBe(200);
    expect((await payResp.json()).status).toBe('pago');
  });

  test('Vendedor NÃO pode aprovar comissão (RBAC — P-IV)', async ({ request, gestorToken, vendedorToken }) => {
    const commID = await createPendingCommission(request, gestorToken);

    const resp = await request.patch(`${API_BASE}/commissions/${commID}/status`, {
      headers: { Authorization: `Bearer ${vendedorToken}` },
      data: { status: 'aprovado', motivo: 'Tentativa indevida' },
    });
    expect(resp.status()).toBe(403);
  });

  test('Gestor NÃO pode aprovar comissão (apenas Financeiro)', async ({ request, gestorToken }) => {
    const commID = await createPendingCommission(request, gestorToken);

    const resp = await request.patch(`${API_BASE}/commissions/${commID}/status`, {
      headers: { Authorization: `Bearer ${gestorToken}` },
      data: { status: 'aprovado', motivo: 'Tentativa de gestor' },
    });
    expect(resp.status()).toBe(403);
  });
});
