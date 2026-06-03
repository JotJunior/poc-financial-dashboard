import { test, expect } from '@playwright/test';

/**
 * Smoke test: API health check
 * Ref: tasks.md §0.4.2; quickstart.md §7
 */
test('API health check returns 200', async ({ request }) => {
  const response = await request.get('http://localhost:8080/health');
  expect(response.status()).toBe(200);
  const body = await response.json();
  expect(body).toHaveProperty('status', 'ok');
});
