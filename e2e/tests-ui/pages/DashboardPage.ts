/**
 * DashboardPage — Page Object para dashboards consolidado e do vendedor.
 * Ref: tasks.md §9.3; DashboardConsolidated.tsx; DashboardVendor.tsx
 */
import { type Page, expect } from '@playwright/test';

export class ConsolidatedDashboardPage {
  constructor(private readonly page: Page) {}

  async goto(): Promise<void> {
    await this.page.goto('/dashboard/consolidated');
    await expect(
      this.page.getByRole('heading', { name: 'Dashboard Consolidado' })
    ).toBeVisible({ timeout: 10_000 });
  }

  async expectMetricCards(): Promise<void> {
    // Verifica que os 5 cartões de métricas aparecem
    await expect(this.page.getByText('Total de Vendas')).toBeVisible({ timeout: 5_000 });
    await expect(this.page.getByText('Comissões Apuradas')).toBeVisible();
    await expect(this.page.getByText('Pendentes de Aprovação')).toBeVisible();
    await expect(this.page.getByText('Aprovadas (a pagar)')).toBeVisible();
    await expect(this.page.getByText('Pagas')).toBeVisible();
  }

  async expectLoaded(): Promise<void> {
    // Aguarda sumir o spinner de carregamento
    await expect(this.page.getByText('Carregando…')).not.toBeVisible({ timeout: 10_000 });
  }
}

export class VendorDashboardPage {
  constructor(private readonly page: Page) {}

  async goto(): Promise<void> {
    await this.page.goto('/dashboard/vendor');
    await expect(
      this.page.getByRole('heading', { name: 'Meu Dashboard' })
    ).toBeVisible({ timeout: 10_000 });
  }

  async expectScopeNotice(): Promise<void> {
    // P-IV: vendedor vê aviso de escopo próprio
    await expect(
      this.page.getByText('Você está vendo apenas seus próprios dados.', { exact: false })
    ).toBeVisible({ timeout: 5_000 });
  }

  async expectMetricCards(): Promise<void> {
    await expect(this.page.getByText('Volume de Vendas')).toBeVisible({ timeout: 5_000 });
    await expect(this.page.getByText('Comissões Apuradas')).toBeVisible();
  }

  async expectLoaded(): Promise<void> {
    await expect(this.page.getByText('Carregando…')).not.toBeVisible({ timeout: 10_000 });
  }
}
