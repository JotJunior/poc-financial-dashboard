/**
 * E2E UI: 01-login.ui.spec.ts
 * Testa o fluxo de login na UI React real (browser-headed).
 *
 * IMPORTANTE — Token em memória:
 * O token JWT é armazenado em memória no módulo client.ts (não em localStorage).
 * Após o login, navegue SEMPRE via links da navbar (React Router = sem page reload =
 * token preservado). O uso de page.goto() após login provoca um full page reload
 * e perde o token.
 *
 * Ref: tasks.md §9.3; quickstart.md §E2E no navegador; Login.tsx; Layout.tsx
 */
import { test, expect } from './fixtures/ui-auth';
import { NavBar } from './pages/NavBar';

test.describe('UI — Login e autenticação', () => {
  test('Formulário de login exibe campos esperados (FinDash, E-mail, Senha)', async ({ page }) => {
    // Não faz login — apenas verifica o formulário estático
    await page.goto('/login');
    await expect(page.getByRole('heading', { name: 'FinDash' })).toBeVisible();
    await expect(page.getByText('Sistema de Comissões')).toBeVisible();
    await expect(page.getByLabel('E-mail')).toBeVisible();
    await expect(page.getByLabel('Senha')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Entrar' })).toBeVisible();
  });

  test('Credenciais inválidas exibem mensagem de erro na UI', async ({ page }) => {
    await page.goto('/login');
    await page.getByLabel('E-mail').fill('usuario-invalido@test.com');
    await page.getByLabel('Senha').fill('SenhaErrada123!');
    await page.getByRole('button', { name: 'Entrar' }).click();

    // Alerta de erro deve aparecer; não deve redirecionar
    await expect(page.getByRole('alert')).toBeVisible({ timeout: 5_000 });
    await expect(page).toHaveURL(/login/, { timeout: 5_000 });
  });

  test('Gestor logado vê navbar completa (Dashboard, Vendedores, Pedidos, Comissões, Pendentes)', async ({ gestorPage: page }) => {
    const navbar = new NavBar(page);

    // Fixture: já logado e no dashboard consolidado
    await expect(page.getByRole('heading', { name: 'Dashboard Consolidado' })).toBeVisible({ timeout: 10_000 });

    // Verificar navbar por papel (Gestor)
    await navbar.expectGestorNav();
    await navbar.expectRoleBadge('Gestor');
  });

  test('Vendedor logado vê apenas links próprios — Meu Dashboard, Pedidos, Minhas Comissões (P-IV)', async ({ vendedorPage: page }) => {
    const navbar = new NavBar(page);

    // Fixture: após login como vendedor, DefaultDashboard redireciona para /dashboard/vendor
    // O layout com navbar está ativo nesta página
    await expect(page.getByRole('heading', { name: 'Meu Dashboard' })).toBeVisible({ timeout: 10_000 });

    // Verificar navbar restrita de vendedor
    await navbar.expectVendedorNav();
    await navbar.expectRoleBadge('Vendedor');
  });

  test('Gestor pode fazer logout e é redirecionado para login', async ({ gestorPage: page }) => {
    const navbar = new NavBar(page);

    // Fixture: já logado — deve ter o botão Sair na navbar (verificado pela fixture)
    // Verificar que o botão de logout está visível (indica navbar carregada)
    await expect(page.getByRole('button', { name: 'Sair' })).toBeVisible({ timeout: 5_000 });

    // Fazer logout via botão na navbar
    await navbar.clickLogout();

    // Deve redirecionar para /login
    await expect(page).toHaveURL(/login/, { timeout: 10_000 });
    await expect(page.getByRole('heading', { name: 'FinDash' })).toBeVisible();
  });
});
