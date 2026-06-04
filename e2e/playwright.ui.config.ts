import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright UI config — Financial Dashboard (browser-driven, headed)
 * Ref: tasks.md §9.3; quickstart.md §E2E no navegador
 *
 * Testes neste config ABREM O NAVEGADOR e navegam a UI React real.
 * Para assistir: cd e2e && npm run test:ui:headed
 *
 * Pré-requisitos:
 *   - Docker Postgres rodando: docker compose up -d postgres
 *   - Backend Go rodando: cd backend && go run ./cmd/api  (ou via webServer abaixo)
 *   - Frontend Vite: cd web && npm run dev  (ou via webServer abaixo)
 *
 * O webServer com reuseExistingServer:true significa:
 *   - Se já estiverem rodando → usa os existentes (dev flow normal)
 *   - Se não estiverem → sobe automaticamente para CI / primeira execução
 */
export default defineConfig({
  testDir: './tests-ui',
  globalSetup: './global-setup-ui.ts',
  fullyParallel: false,         // Sequencial: evita conflitos de estado no banco
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  reporter: [['list'], ['html', { outputFolder: 'playwright-report-ui', open: 'never' }]],
  timeout: 60_000,             // UI pode ser mais lenta que API pura

  use: {
    baseURL: process.env.BASE_URL || 'http://localhost:5173',
    headless: process.env.HEADLESS === 'true' ? true : false,  // headed por padrão
    viewport: { width: 1280, height: 720 },
    video: 'on',               // Sempre gravar — operador pode rever
    trace: 'on',               // Trace sempre ligado para debugging
    screenshot: 'only-on-failure',
    actionTimeout: 15_000,     // Cada ação (click, fill) tem 15s
    navigationTimeout: 30_000, // Navegações têm 30s
  },

  projects: [
    {
      name: 'chromium-headed',
      use: { ...devices['Desktop Chrome'] },
    },
  ],

  /**
   * webServer — sobe backend e frontend automaticamente se não estiverem rodando.
   * reuseExistingServer:true significa que se já estiverem no ar, não derruba.
   *
   * Para CI puro sem servidores pre-existentes, defina:
   *   HEADLESS=true npx playwright test --config=playwright.ui.config.ts
   */
  webServer: [
    {
      // Backend Go — porta 8080
      // RATE_LIMIT_MAX=100: aumenta o rate limit de login de 5 para 100 req/60s
      // para que os testes UI não sejam bloqueados durante execução sequencial.
      command: 'cd ../backend && DATABASE_URL="postgres://financialuser:financialpass@localhost:5433/financial_dashboard?sslmode=disable" JWT_SECRET="e2e-test-secret-change-in-prod" JWT_ACCESS_TTL="1h" JWT_REFRESH_TTL="168h" RATE_LIMIT_MAX=100 go run ./cmd/api',
      url: 'http://localhost:8080/health',
      reuseExistingServer: true,
      timeout: 30_000,
      stdout: 'pipe',
      stderr: 'pipe',
    },
    {
      // Frontend Vite — porta 5173
      command: 'cd ../web && npm run dev -- --port 5173',
      url: 'http://localhost:5173',
      reuseExistingServer: true,
      timeout: 30_000,
      stdout: 'pipe',
      stderr: 'pipe',
    },
  ],
});
