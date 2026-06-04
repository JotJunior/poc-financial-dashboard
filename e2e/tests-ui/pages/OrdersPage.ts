/**
 * OrdersPage — Page Object para a tela de Pedidos.
 * Ref: tasks.md §9.3; Orders.tsx; OrderForm.tsx; OrderStatusBadge.tsx
 */
import { type Page, expect } from '@playwright/test';

export class OrdersPage {
  constructor(private readonly page: Page) {}

  async goto(): Promise<void> {
    await this.page.goto('/orders');
    await expect(
      this.page.getByRole('heading', { name: 'Pedidos' })
    ).toBeVisible({ timeout: 10_000 });
  }

  async openCreateForm(): Promise<void> {
    await this.page.getByRole('button', { name: '+ Novo Pedido' }).click();
    await expect(
      this.page.getByRole('heading', { name: 'Novo Pedido' })
    ).toBeVisible({ timeout: 5_000 });
  }

  async selectVendor(vendorName: string): Promise<void> {
    // O formulário tem um <select> com os vendedores ativos
    const vendorSelect = this.page.locator('select').first();
    await vendorSelect.selectOption({ label: vendorName });
  }

  async fillOrderDate(date: string): Promise<void> {
    // Campo de data do pedido
    const dateInput = this.page.locator('input[type="date"]').first();
    await dateInput.fill(date);
  }

  async addItem(opts: {
    description: string;
    quantity: string;
    unitPrice: string;
  }): Promise<void> {
    // Preencher campos do item (são inputs de texto dentro do formulário)
    const descInputs = this.page.locator('input[placeholder*="Produto"]');
    await descInputs.first().fill(opts.description);

    const qtyInputs = this.page.locator('input[placeholder*="1"]').first();
    await qtyInputs.fill(opts.quantity);

    const priceInputs = this.page.locator('input[placeholder*="0.00"]').first();
    await priceInputs.fill(opts.unitPrice);
  }

  async submitOrderForm(): Promise<void> {
    await this.page.getByRole('button', { name: 'Criar Pedido' }).click();
  }

  async expectOrderInList(): Promise<void> {
    // Aguarda pelo menos um pedido aparecer na lista
    await expect(
      this.page.locator('[style*="border-radius: 10px"]').first()
    ).toBeVisible({ timeout: 10_000 });
  }

  async expectStatusBadge(status: string): Promise<void> {
    await expect(this.page.getByText(status, { exact: false })).toBeVisible({ timeout: 5_000 });
  }

  /**
   * Clica no badge de status interativo para transicionar para o próximo estado.
   * O OrderStatusBadge exibe dropdown de transição quando `interactive=true`.
   */
  async transitionFirstOrder(toStatus: string): Promise<void> {
    // Clica no badge de status (botão interativo para gestor)
    const badge = this.page.getByRole('button', { name: /rascunho|confirmado|pago/i }).first();
    await badge.click();
    // Seleciona a opção do menu dropdown
    await this.page.getByRole('option', { name: new RegExp(toStatus, 'i') }).click();
  }
}
