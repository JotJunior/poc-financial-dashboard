/**
 * LoginPage — Page Object para a tela de login do FinDash.
 * Usa getByLabel e getByRole para locators robustos (não seletores CSS frágeis).
 * Ref: tasks.md §9.3
 */
import { type Page, expect } from '@playwright/test';

export class LoginPage {
  constructor(private readonly page: Page) {}

  async goto(): Promise<void> {
    await this.page.goto('/login');
    await expect(this.page.getByRole('heading', { name: 'FinDash' })).toBeVisible();
  }

  async fillEmail(email: string): Promise<void> {
    await this.page.getByLabel('E-mail').fill(email);
  }

  async fillPassword(password: string): Promise<void> {
    await this.page.getByLabel('Senha').fill(password);
  }

  async submit(): Promise<void> {
    await this.page.getByRole('button', { name: 'Entrar' }).click();
  }

  async login(email: string, password: string): Promise<void> {
    await this.goto();
    await this.fillEmail(email);
    await this.fillPassword(password);
    await this.submit();
    // Aguarda sair da página de login (redirect após autenticação)
    await this.page.waitForURL(url => !url.pathname.includes('/login'), {
      timeout: 15_000,
    });
  }

  async expectError(): Promise<void> {
    await expect(
      this.page.getByRole('alert')
    ).toBeVisible({ timeout: 5_000 });
  }
}
