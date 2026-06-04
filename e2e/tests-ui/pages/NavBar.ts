/**
 * NavBar — Page Object para a barra de navegação do FinDash.
 * Verifica links visíveis por papel (role-based nav).
 * Ref: tasks.md §9.3; Layout.tsx
 */
import { type Page, expect } from '@playwright/test';

export class NavBar {
  constructor(private readonly page: Page) {}

  async expectGestorNav(): Promise<void> {
    // Gestor vê: Dashboard, Vendedores, Pedidos, Comissões, Pendentes
    await expect(this.page.getByRole('link', { name: 'Dashboard' })).toBeVisible();
    await expect(this.page.getByRole('link', { name: 'Vendedores' })).toBeVisible();
    await expect(this.page.getByRole('link', { name: 'Pedidos' })).toBeVisible();
    await expect(this.page.getByRole('link', { name: 'Comissões' })).toBeVisible();
    await expect(this.page.getByRole('link', { name: 'Pendentes' })).toBeVisible();
  }

  async expectVendedorNav(): Promise<void> {
    // Vendedor vê: Meu Dashboard, Pedidos, Minhas Comissões — NÃO vê Vendedores nem Pendentes
    await expect(this.page.getByRole('link', { name: 'Meu Dashboard' })).toBeVisible({ timeout: 10_000 });
    await expect(this.page.getByRole('link', { name: 'Pedidos' })).toBeVisible({ timeout: 5_000 });
    await expect(this.page.getByRole('link', { name: 'Minhas Comissões' })).toBeVisible({ timeout: 5_000 });
    // Itens de gestor NÃO devem aparecer
    await expect(this.page.getByRole('link', { name: 'Vendedores' })).not.toBeVisible();
    await expect(this.page.getByRole('link', { name: 'Pendentes' })).not.toBeVisible();
  }

  async expectRoleBadge(role: 'Gestor' | 'Vendedor' | 'Financeiro'): Promise<void> {
    // Badge colorido com o papel do usuário
    await expect(this.page.getByText(role, { exact: true })).toBeVisible();
  }

  async clickLogout(): Promise<void> {
    await this.page.getByRole('button', { name: 'Sair' }).click();
  }

  async navigateTo(link: string): Promise<void> {
    await this.page.getByRole('link', { name: link }).click();
  }
}
