/**
 * VendorsPage — Page Object para a tela de Vendedores.
 * Ref: tasks.md §9.3; Vendors.tsx; VendorForm.tsx
 */
import { type Page, expect } from '@playwright/test';

export class VendorsPage {
  constructor(private readonly page: Page) {}

  async goto(): Promise<void> {
    await this.page.goto('/vendors');
    await expect(
      this.page.getByRole('heading', { name: 'Vendedores' })
    ).toBeVisible({ timeout: 10_000 });
  }

  async openCreateForm(): Promise<void> {
    await this.page.getByRole('button', { name: '+ Novo Vendedor' }).click();
    await expect(
      this.page.getByRole('heading', { name: 'Novo Vendedor' })
    ).toBeVisible();
  }

  async fillVendorForm(opts: {
    name: string;
    email: string;
    percentage: string;
    validFrom?: string;
  }): Promise<void> {
    await this.page.getByLabel('Nome *').fill(opts.name);
    await this.page.getByLabel('E-mail *').fill(opts.email);
    await this.page.getByLabel(/Percentual de comissão/i).fill(opts.percentage);
    if (opts.validFrom) {
      await this.page.getByLabel('Vigência a partir de *').fill(opts.validFrom);
    }
  }

  async submitVendorForm(): Promise<void> {
    await this.page.getByRole('button', { name: 'Criar Vendedor' }).click();
  }

  async expectVendorInList(name: string): Promise<void> {
    // Aguarda o vendedor aparecer na lista
    await expect(
      this.page.getByText(name, { exact: false })
    ).toBeVisible({ timeout: 10_000 });
  }

  async expectVendorCount(text: string): Promise<void> {
    await expect(this.page.getByText(text, { exact: false })).toBeVisible();
  }
}
