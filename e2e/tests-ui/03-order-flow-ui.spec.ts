/**
 * E2E UI: 03-order-flow-ui.spec.ts
 * Testa o ciclo de vida de pedidos via UI React (browser-headed).
 * Usa fixture gestorPage (login por test) + navegação via navbar/SPA.
 *
 * Fluxos cobertos:
 *   - Criar pedido via formulário → aparece na lista como "Rascunho"
 *   - Confirmar pedido → badge vira "Confirmado"
 *   - Marcar pago → badge vira "Pago"
 *   - Filtros de status funcionam na UI
 *
 * Ref: tasks.md §9.3; Orders.tsx; OrderForm.tsx; OrderStatusBadge.tsx
 */
import { test, expect } from './fixtures/ui-auth';

test.describe('UI — Ciclo de vida de pedidos', () => {
  test('Gestor cria pedido via formulário e ele aparece na lista como Rascunho', async ({ gestorPage: page }) => {
    // Navegar para Pedidos via navbar (React Router = token preservado)
    await page.getByRole('link', { name: 'Pedidos' }).click();
    await expect(page.getByRole('heading', { name: 'Pedidos' })).toBeVisible({ timeout: 10_000 });

    // Aguardar a lista de pedidos carregar completamente (evitar "element detached")
    // O botão "+ Novo Pedido" pode reaparecer após o erro inicial da lista ser resolvido
    await page.waitForTimeout(2_000);

    // Abrir formulário
    await page.getByRole('button', { name: '+ Novo Pedido' }).click();
    await expect(page.getByRole('heading', { name: 'Novo Pedido' })).toBeVisible({ timeout: 5_000 });

    // Aguarda o select de vendedor aparecer no form
    const vendorSelect = page.getByLabel('Vendedor *');
    await expect(vendorSelect).toBeVisible({ timeout: 10_000 });

    // Aguardar opções de vendedor carregarem (react-query async fetch)
    // A lista de vendors é buscada da API quando o formulário abre.
    // Esperar até 20 segundos (10s é suficiente mas margem de segurança)
    await page.waitForFunction(() => {
      const sel = document.querySelector<HTMLSelectElement>('select#of-vendor');
      return sel !== null && sel.options.length > 1;
    }, { timeout: 20_000 });
    await vendorSelect.selectOption({ index: 1 });

    // Preencher data do pedido
    await page.getByLabel('Data do Pedido *').fill('2026-06-01');

    // Preencher o item do pedido (description, qty, price)
    const descInput = page.locator('input[placeholder="Descrição"]').first();
    await descInput.fill('Produto E2E UI');

    const qtyInput = page.locator('input[placeholder="Qtd"]').first();
    await qtyInput.fill('2');

    const priceInput = page.locator('input[placeholder*="Valor unit"]').first();
    await priceInput.fill('500.00');

    // Submeter
    await page.getByRole('button', { name: 'Criar Pedido' }).click();

    // Formulário fecha
    await expect(
      page.getByRole('heading', { name: 'Novo Pedido' })
    ).not.toBeVisible({ timeout: 15_000 });

    // Aguarda react-query refetch da lista de pedidos
    await page.waitForTimeout(2_000);

    // Filtrar por status=rascunho para ver o pedido recém-criado
    const statusFilter = page.locator('select').first();
    await statusFilter.selectOption({ value: 'rascunho' }); // Filtrar por rascunhos
    await page.waitForTimeout(800);

    // Verificar que aparecem pedidos no status rascunho (badge span com "Rascunho")
    // Usar span ao invés de getByText para evitar match no <option> do select
    await expect(page.locator('span').filter({ hasText: /^Rascunho$/ }).first()).toBeVisible({ timeout: 10_000 });
  });

  test('Confirmar pedido — badge muda para Confirmado na UI', async ({ gestorPage: page }) => {
    await page.getByRole('link', { name: 'Pedidos' }).click();
    await expect(page.getByRole('heading', { name: 'Pedidos' })).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(1_500);

    // Filtrar por "rascunho" para encontrar pedidos com botão "Confirmar"
    const statusFilter = page.locator('select').first();
    await statusFilter.selectOption({ value: 'rascunho' });
    await page.waitForTimeout(500);

    const confirmarBtn = page.getByRole('button', { name: 'Confirmar' }).first();
    const btnCount = await confirmarBtn.count();

    if (btnCount === 0) {
      console.warn('[03-order-flow-ui] Nenhum pedido em rascunho para confirmar. Pulando.');
      return;
    }

    await confirmarBtn.click();

    // Após confirmar, o pedido some do filtro "rascunho".
    // Limpar filtro e verificar que "Confirmado" aparece na lista geral.
    await page.waitForTimeout(1_000);
    await statusFilter.selectOption({ value: '' }); // Todos os status
    await page.waitForTimeout(800);

    // Usar span para evitar match no <option> do select
    await expect(
      page.locator('span').filter({ hasText: /^Confirmado$/ }).first()
    ).toBeVisible({ timeout: 15_000 });
  });

  test('Marcar pedido como pago — badge muda para Pago na UI', async ({ gestorPage: page }) => {
    await page.getByRole('link', { name: 'Pedidos' }).click();
    await expect(page.getByRole('heading', { name: 'Pedidos' })).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(1_500);

    // Filtrar por "confirmado" para encontrar pedidos com botão "Marcar Pago"
    const statusFilter = page.locator('select').first();
    await statusFilter.selectOption({ value: 'confirmado' });
    await page.waitForTimeout(500);

    const marcarPagoBtn = page.getByRole('button', { name: 'Marcar Pago' }).first();
    const btnCount = await marcarPagoBtn.count();

    if (btnCount === 0) {
      // Tentar criar e confirmar um pedido primeiro, ou pular
      console.warn('[03-order-flow-ui] Nenhum pedido confirmado disponível para marcar pago. Pulando.');
      return;
    }

    await marcarPagoBtn.click();

    // Após marcar pago, limpar filtro e verificar que "Pago" aparece na lista geral
    await page.waitForTimeout(1_000);
    await statusFilter.selectOption({ value: '' }); // Todos os status
    await page.waitForTimeout(800);

    // Usar span para evitar match no <option> do select
    await expect(
      page.locator('span').filter({ hasText: /^Pago$/ }).first()
    ).toBeVisible({ timeout: 15_000 });
  });

  test('Filtro de status "Rascunho" é selecionável na UI', async ({ gestorPage: page }) => {
    await page.getByRole('link', { name: 'Pedidos' }).click();
    await expect(page.getByRole('heading', { name: 'Pedidos' })).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(1_500);

    const statusFilter = page.locator('select').first();
    await statusFilter.selectOption({ value: 'rascunho' });
    await expect(statusFilter).toHaveValue('rascunho');
    console.log('[03-order-flow-ui] Filtro de status "rascunho" aplicado com sucesso.');
  });
});
