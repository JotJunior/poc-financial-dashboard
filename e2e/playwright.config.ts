import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright E2E config — Financial Dashboard
 * Ref: quickstart.md §7 Testes; tasks.md §0.4.2; tasks.md §9.2
 *
 * Os testes E2E usam a API diretamente (via request fixture) sem interação de UI,
 * o que os torna mais estáveis e rápidos. Para rodar:
 *
 *   API_BASE_URL=http://localhost:8080/api/v1 npx playwright test
 *
 * Pré-requisito: backend rodando em localhost:8080 e banco populado com seed E2E.
 */
export default defineConfig({
  testDir: './tests',
  globalSetup: './global-setup.ts',
  fullyParallel: false, // sequential para evitar rate limit (5 req/60s por IP)
  forbidOnly: !!process.env.CI,
  retries: 0,
  workers: 1,
  reporter: 'list',
  timeout: 30000, // 30s por teste

  use: {
    // URL base do frontend (dev server)
    baseURL: process.env.BASE_URL || 'http://localhost:5173',
    trace: 'on-first-retry',
    // Para testes que usam apenas a API (sem UI), definir API base via env
    extraHTTPHeaders: {
      'Accept': 'application/json',
    },
  },

  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
});
