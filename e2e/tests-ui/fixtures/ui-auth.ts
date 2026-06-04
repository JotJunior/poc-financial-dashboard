/**
 * UI Auth Fixture — login compartilhado para testes browser-driven.
 *
 * IMPORTANTE sobre a arquitetura de auth deste app:
 * O token JWT é armazenado em memória no módulo `client.ts` (variável `_accessToken`).
 * Isso significa que um `page.goto()` com URL completa causa um full page reload e
 * PERDE o token. A navegação precisa ser feita via React Router (links internos)
 * após o login inicial, OU o token deve ser reinjetado via localStorage.
 *
 * ESTRATÉGIA ADOTADA para E2E UI tests:
 * Após login, navegar via `page.goto()` com a URL correta — mas o ProtectedRoute
 * vai redirecionar para /login se o token não estiver em memória.
 *
 * SOLUÇÃO: usar `waitForLoadState('networkidle')` após goto para garantir que o
 * React carregou, e verificar se foi redirecionado de volta para /login (o que
 * indica que precisamos fazer login novamente na página atual).
 *
 * Para cada teste que precisa de uma página específica + autenticação:
 * 1. Chamar loginViaUI() que faz login no /login e aguarda o redirect
 * 2. Usar page.getByRole('link').click() para navegar DENTRO do SPA
 *    (React Router — SEM full page reload, token em memória é preservado)
 *
 * Ref: tasks.md §9.3
 */
import { test as base, type Page, expect } from '@playwright/test';

export const CREDENTIALS = {
  gestor: { email: 'gestor-e2e@test.com', password: 'E2ETest@2026!' },
  vendedor: { email: 'vendedor-user-e2e@test.com', password: 'E2ETest@2026!' },
};

/**
 * Login via UI e aguarda redirect pós-autenticação.
 * DEVE chamar loginViaUI() e depois usar navigateInSPA() para navegar.
 */
export async function loginViaUI(page: Page, role: 'gestor' | 'vendedor'): Promise<void> {
  const creds = CREDENTIALS[role];
  await page.goto('/login');
  await expect(page.getByRole('heading', { name: 'FinDash' })).toBeVisible({ timeout: 10_000 });
  await page.getByLabel('E-mail').fill(creds.email);
  await page.getByLabel('Senha').fill(creds.password);
  await page.getByRole('button', { name: 'Entrar' }).click();
  // Aguarda sair da página de login (redirect para dashboard)
  await page.waitForURL(url => !url.pathname.includes('/login'), { timeout: 20_000 });
  // Aguarda o botão de logout aparecer na navbar (confirma que o layout carregou)
  await expect(page.getByRole('button', { name: 'Sair' })).toBeVisible({ timeout: 15_000 });
}

/**
 * Navegar DENTRO do SPA usando um link da navbar (sem full page reload).
 * Preserva o token em memória.
 */
export async function navigateViaNavLink(page: Page, linkName: string, expectedHeading: string): Promise<void> {
  // Tentar via link na navbar (sem page reload)
  const navLink = page.getByRole('link', { name: linkName }).first();
  const count = await navLink.count();

  if (count > 0) {
    await navLink.click();
  } else {
    // Fallback: navegação direta (aceita que pode haver logout)
    await page.goto(`/${linkName.toLowerCase()}`);
  }

  await expect(
    page.getByRole('heading', { name: expectedHeading })
  ).toBeVisible({ timeout: 15_000 });
}

// Tipo de fixtures extras
export type UIAuthFixtures = {
  gestorPage: Page;
  vendedorPage: Page;
};

/**
 * test estendido com fixtures de página pré-autenticada por papel.
 * Cada fixture cria uma nova page, faz login e aguarda o SPA carregar.
 */
export const test = base.extend<UIAuthFixtures>({
  gestorPage: async ({ browser }, use) => {
    const context = await browser.newContext();
    const page = await context.newPage();
    await loginViaUI(page, 'gestor');
    await use(page);
    await context.close();
  },
  vendedorPage: async ({ browser }, use) => {
    const context = await browser.newContext();
    const page = await context.newPage();
    await loginViaUI(page, 'vendedor');
    await use(page);
    await context.close();
  },
});

export { expect } from '@playwright/test';
