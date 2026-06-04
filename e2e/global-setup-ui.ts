/**
 * Global setup para testes UI (browser-driven).
 *
 * IMPORTANTE — Rate limit do backend: 5 logins/60s por IP.
 * Por isso este setup NÃO faz login — apenas verifica conectividade.
 * Os arquivos de teste usam beforeAll para login compartilhado.
 *
 * Credenciais de seed UI:
 *   Gestor:   gestor-e2e@test.com       / E2ETest@2026!
 *   Vendedor: vendedor-user-e2e@test.com / E2ETest@2026!
 *
 * Ref: tasks.md §9.3; quickstart.md §E2E no navegador
 */
import { request } from '@playwright/test';

const API_BASE = process.env.API_BASE_URL || 'http://localhost:8080/api/v1';

export const UI_CREDENTIALS = {
  gestor: { email: 'gestor-e2e@test.com', password: 'E2ETest@2026!' },
  vendedor: { email: 'vendedor-user-e2e@test.com', password: 'E2ETest@2026!' },
};

export default async function globalSetupUI(): Promise<void> {
  console.log('\n[global-setup-ui] Verificando conectividade com backend...');

  const ctx = await request.newContext();
  try {
    const health = await ctx.get(`${API_BASE.replace('/api/v1', '')}/health`);
    if (health.status() !== 200) {
      throw new Error(`[global-setup-ui] Backend não responde: HTTP ${health.status()}`);
    }
    console.log('[global-setup-ui] Backend OK. Rate limit preservado para testes.');
    console.log('[global-setup-ui] Credenciais E2E:');
    console.log(`  Gestor:   ${UI_CREDENTIALS.gestor.email} / ${UI_CREDENTIALS.gestor.password}`);
    console.log(`  Vendedor: ${UI_CREDENTIALS.vendedor.email} / ${UI_CREDENTIALS.vendedor.password}\n`);
  } finally {
    await ctx.dispose();
  }
}
