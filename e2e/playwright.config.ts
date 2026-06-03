import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright E2E config — Financial Dashboard
 * Ref: quickstart.md §7 Testes; tasks.md §0.4.2
 */
export default defineConfig({
  testDir: './tests',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: 'html',

  use: {
    // URL base do frontend (dev server)
    baseURL: process.env.BASE_URL || 'http://localhost:5173',
    trace: 'on-first-retry',
  },

  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],

  // Iniciar dev servers antes dos testes (se disponível)
  // webServer: {
  //   command: 'cd ../web && npm run dev',
  //   url: 'http://localhost:5173',
  //   reuseExistingServer: !process.env.CI,
  // },
});
