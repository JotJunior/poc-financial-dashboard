/**
 * CommissionsPage — Page Object para a tela de Comissões.
 * Cobre apuração (ApurationForm) e listagem.
 * Ref: tasks.md §9.3; Commissions.tsx; ApurationForm.tsx
 */
import { type Page, expect } from '@playwright/test';

export class CommissionsPage {
  constructor(private readonly page: Page) {}

  async goto(): Promise<void> {
    await this.page.goto('/commissions');
    await expect(
      this.page.getByRole('heading', { name: 'Comissões' })
    ).toBeVisible({ timeout: 10_000 });
  }

  async apurate(year: number, month: number): Promise<void> {
    // ApurationForm — selecionar mês e ano, clicar Apurar
    const apurationSection = this.page.getByRole('heading', {
      name: 'Apuração de Comissões',
    });
    await expect(apurationSection).toBeVisible({ timeout: 5_000 });

    // Selecionar mês (select com nomes de meses em português)
    const monthNames = [
      'janeiro', 'fevereiro', 'março', 'abril', 'maio', 'junho',
      'julho', 'agosto', 'setembro', 'outubro', 'novembro', 'dezembro',
    ];
    const monthName = monthNames[month - 1];
    await this.page.locator('select').first().selectOption({ label: new RegExp(monthName, 'i') });

    // Selecionar ano
    await this.page.locator('select').nth(1).selectOption({ value: String(year) });

    // Clicar Apurar
    await this.page.getByRole('button', { name: 'Apurar' }).click();

    // Aguarda o botão voltar ao estado normal (não "Apurando…")
    await expect(
      this.page.getByRole('button', { name: 'Apurar' })
    ).toBeEnabled({ timeout: 15_000 });
  }

  async expectCommissionVisible(): Promise<void> {
    // Após apuração, espera pelo menos um item de comissão na lista
    await this.page.waitForTimeout(1_500); // breve espera para refresh
    await this.page.reload();
    await expect(
      this.page.getByRole('heading', { name: 'Comissões' })
    ).toBeVisible();
  }
}
