/**
 * E2E UI: 05-dashboard-ui.spec.ts
 * Testa o Dashboard Consolidado (Gestor) e Dashboard do Vendedor.
 * Usa fixtures gestorPage/vendedorPage + navegação via SPA.
 *
 * Fluxos cobertos:
 *   - Dashboard Consolidado: métricas carregam (5 cartões)
 *   - Dashboard Vendedor: scope notice de "apenas seus próprios dados" (P-IV)
 *   - Vendedor NÃO pode ver /dashboard/consolidated (bloqueio por papel)
 *   - Gestor navega por todas as seções sem erro
 *
 * Ref: tasks.md §9.3; DashboardConsolidated.tsx; DashboardVendor.tsx; Layout.tsx
 */
import { test, expect } from './fixtures/ui-auth';
import { ConsolidatedDashboardPage, VendorDashboardPage } from './pages/DashboardPage';

test.describe('UI — Dashboard Consolidado (Gestor)', () => {
  test('Dashboard Consolidado carrega métricas (5 cartões)', async ({ gestorPage: page }) => {
    // Fixture já fez login e estamos no dashboard consolidado
    // Verificar métricas (a fixture termina após o login redirect)
    await expect(page.getByText('Total de Vendas')).toBeVisible({ timeout: 10_000 });
    await expect(page.getByText('Comissões Apuradas')).toBeVisible();
    await expect(page.getByText('Pendentes de Aprovação')).toBeVisible();
    await expect(page.getByText('Aprovadas (a pagar)')).toBeVisible();
    await expect(page.getByText('Pagas')).toBeVisible();
  });

  test('Dashboard Consolidado tem filtros de período (ano/mês)', async ({ gestorPage: page }) => {
    // Já logado no dashboard consolidado via fixture
    const dashboard = new ConsolidatedDashboardPage(page);

    // Filtros de período devem estar presentes
    const selects = page.locator('select');
    await expect(selects.first()).toBeVisible({ timeout: 5_000 });

    // Selecionar ano 2026
    await selects.first().selectOption({ value: '2026' });

    // Métricas atualizam
    await dashboard.expectLoaded();
    await dashboard.expectMetricCards();
  });
});

test.describe('UI — Dashboard do Vendedor (P-IV isolamento)', () => {
  test('Vendedor vê "Meu Dashboard" com aviso de escopo próprio (P-IV)', async ({ vendedorPage: page }) => {
    // Fixture: após login, o vendedor está em /dashboard/vendor (DefaultDashboard fix)
    // Navegar explicitamente via navbar para garantir que a página carrega fresca
    const myDashLink = page.getByRole('link', { name: 'Meu Dashboard' }).first();
    if (await myDashLink.count() > 0) {
      await myDashLink.click();
    }

    await expect(page.getByRole('heading', { name: 'Meu Dashboard' })).toBeVisible({ timeout: 10_000 });

    // Aguardar spinner desaparecer (dados carregando via react-query)
    await expect(page.getByText('Carregando…')).not.toBeVisible({ timeout: 20_000 });

    // Verificar cartões de métricas
    await expect(page.getByText('Volume de Vendas')).toBeVisible({ timeout: 10_000 });
    await expect(page.getByText('Comissões Apuradas')).toBeVisible({ timeout: 5_000 });

    // Aviso de isolamento (P-IV)
    await expect(
      page.getByText('Você está vendo apenas seus próprios dados.', { exact: false })
    ).toBeVisible({ timeout: 5_000 });
  });

  test('Vendedor NÃO pode acessar /dashboard/consolidated — recebe bloqueio via RBAC', async ({ vendedorPage: page }) => {
    // Estratégia: usar navegação DENTRO do SPA (link, não goto) para preservar o token.
    // O ProtectedRoute roles=['gestor','financeiro'] deve redirecionar vendedor para /403.
    // Como não há link direto para /dashboard/consolidated no menu de vendedor,
    // testamos via window.history.pushState (sem reload):
    await page.evaluate(() => window.history.pushState({}, '', '/dashboard/consolidated'));
    // Acionar renavegação do React Router (pushState não dispara sozinho no React Router 6):
    await page.evaluate(() => window.dispatchEvent(new PopStateEvent('popstate', { state: {} })));

    // Aguardar React Router processar e redirecionar
    await page.waitForTimeout(1000);

    // O ProtectedRoute (roles=['gestor','financeiro']) bloqueia vendedor → redireciona para /403
    const url = page.url();
    const has403 = url.includes('/403') || await page.getByRole('heading', { name: '403' }).count() > 0;
    const hasErrorMsg = await page.getByText('Sem permissão', { exact: false }).count() > 0;
    const hasApiError = await page.getByText('Sem permissão ou erro ao carregar dashboard', { exact: false }).count() > 0;
    const onLoginPage = url.includes('/login');

    const isBlocked = has403 || hasErrorMsg || hasApiError || onLoginPage;
    expect(isBlocked, `Vendedor deveria ser bloqueado no /dashboard/consolidated. URL: ${url}`).toBe(true);
    console.log(`[05-dashboard-ui] Acesso bloqueado como esperado. URL final: ${url}`);
  });
});

test.describe('UI — Navegação completa ponta-a-ponta (Gestor)', () => {
  test('Gestor navega por todas as seções principais sem erros', async ({ gestorPage: page }) => {
    // Já logado. Navegar pelo SPA via links da navbar.

    // Dashboard já carregado (post-login redirect)
    await expect(page.getByRole('heading', { name: 'Dashboard Consolidado' })).toBeVisible({ timeout: 10_000 });

    await page.getByRole('link', { name: 'Vendedores' }).click();
    await expect(page.getByRole('heading', { name: 'Vendedores' })).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(800);

    await page.getByRole('link', { name: 'Pedidos' }).click();
    await expect(page.getByRole('heading', { name: 'Pedidos' })).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(2_000); // Pedidos tem mais carga (useOrders + useVendors)

    await page.getByRole('link', { name: 'Comissões' }).click();
    await expect(page.getByRole('heading', { name: 'Comissões', exact: true })).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(800);

    await page.getByRole('link', { name: 'Pendentes' }).click();
    await expect(page.getByRole('heading', { name: 'Comissões Pendentes' })).toBeVisible({ timeout: 10_000 });

    console.log('[05-dashboard-ui] Navegação completa por todas as seções: OK');
  });
});
