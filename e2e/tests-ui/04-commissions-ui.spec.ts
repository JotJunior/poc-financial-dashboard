/**
 * E2E UI: 04-commissions-ui.spec.ts
 * Testa apuração de comissões e visualização do saldo via UI React.
 * Usa fixture gestorPage (login por test) + navegação via SPA.
 *
 * Fluxos cobertos:
 *   - Gestor acessa tela de Comissões e vê ApurationForm
 *   - Gestor apura comissões de um período → botão volta ao normal
 *   - Comissões Pendentes exibe indicadores de saldo
 *
 * Ref: tasks.md §9.3; Commissions.tsx; ApurationForm.tsx; PendingCommissions.tsx
 */
import { test, expect } from './fixtures/ui-auth';

test.describe('UI — Comissões e apuração', () => {
  test('Tela de comissões exibe seção de apuração (apenas Gestor)', async ({ gestorPage: page }) => {
    // Navegar via navbar (React Router = token preservado)
    await page.getByRole('link', { name: 'Comissões' }).click();
    await expect(page.getByRole('heading', { name: 'Comissões', exact: true })).toBeVisible({ timeout: 10_000 });

    // ApurationForm deve estar visível para o Gestor
    await expect(
      page.getByRole('heading', { name: 'Apuração de Comissões' })
    ).toBeVisible({ timeout: 5_000 });

    await expect(page.getByRole('button', { name: 'Apurar' })).toBeVisible();
  });

  test('Gestor apura comissões de junho/2026 — botão volta ao estado normal', async ({ gestorPage: page }) => {
    await page.getByRole('link', { name: 'Comissões' }).click();
    await expect(page.getByRole('heading', { name: 'Comissões', exact: true })).toBeVisible({ timeout: 10_000 });
    await expect(page.getByRole('heading', { name: 'Apuração de Comissões' })).toBeVisible({ timeout: 5_000 });

    // Selecionar mês junho e ano 2026 nos selects do ApurationForm
    await page.locator('select').first().selectOption({ label: 'junho' });
    await page.locator('select').nth(1).selectOption({ value: '2026' });

    // Clicar em Apurar
    await page.getByRole('button', { name: 'Apurar' }).click();

    // Botão volta ao estado "Apurar"
    await expect(
      page.getByRole('button', { name: 'Apurar' })
    ).toBeEnabled({ timeout: 20_000 });

    console.log('[04-commissions-ui] Apuração de jun/2026 concluída via UI.');
  });

  test('Tela de Comissões Pendentes exibe indicadores de saldo', async ({ gestorPage: page }) => {
    // Navegar via navbar
    await page.getByRole('link', { name: 'Pendentes' }).click();
    await expect(
      page.getByRole('heading', { name: 'Comissões Pendentes' })
    ).toBeVisible({ timeout: 10_000 });

    await expect(page.getByText('Pendentes de Aprovação')).toBeVisible({ timeout: 5_000 });
    await expect(page.getByText('Aprovadas — Aguardando Pagamento')).toBeVisible();
    await expect(page.getByRole('link', { name: 'Ver Pendentes' })).toBeVisible();
    await expect(page.getByRole('link', { name: 'Ver Aprovadas' })).toBeVisible();
  });

  test('Filtros de status na lista de comissões são selecionáveis', async ({ gestorPage: page }) => {
    await page.getByRole('link', { name: 'Comissões' }).click();
    await expect(page.getByRole('heading', { name: 'Comissões', exact: true })).toBeVisible({ timeout: 10_000 });

    // Procurar o select com option value="pendente" (filtro de status)
    const selects = page.locator('select');
    const count = await selects.count();

    for (let i = 0; i < count; i++) {
      const opts = await selects.nth(i).locator('option[value="pendente"]').count();
      if (opts > 0) {
        await selects.nth(i).selectOption({ value: 'pendente' });
        await expect(selects.nth(i)).toHaveValue('pendente');
        console.log(`[04-commissions-ui] Filtro "pendente" aplicado no select ${i}.`);
        break;
      }
    }
  });
});
