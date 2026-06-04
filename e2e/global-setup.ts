import { request, FullConfig } from '@playwright/test';
import { writeFileSync } from 'fs';

const API_BASE = process.env.API_BASE_URL || 'http://localhost:8080/api/v1';
const TOKEN_FILE = '/tmp/e2e-tokens.json';

async function loginUser(email: string, password: string): Promise<string> {
  const ctx = await request.newContext();
  const resp = await ctx.post(`${API_BASE}/auth/login`, { data: { email, password } });
  if (resp.status() !== 200) {
    const text = await resp.text();
    await ctx.dispose();
    throw new Error(`Login failed for ${email} (${resp.status()}): ${text}`);
  }
  const body = await resp.json();
  await ctx.dispose();
  return body.access_token as string;
}

async function globalSetup(_config: FullConfig): Promise<void> {
  console.log('\n[global-setup] Gerando tokens E2E...');
  const gestorToken = await loginUser('gestor-e2e@test.com', 'E2ETest@2026!');
  console.log('[global-setup] Gestor OK');
  await new Promise(resolve => setTimeout(resolve, 1000));
  const financeiroToken = await loginUser('financeiro-e2e@test.com', 'E2ETest@2026!');
  console.log('[global-setup] Financeiro OK');
  await new Promise(resolve => setTimeout(resolve, 1000));
  const vendedorToken = await loginUser('vendedor-user-e2e@test.com', 'E2ETest@2026!');
  console.log('[global-setup] Vendedor OK');
  writeFileSync(TOKEN_FILE, JSON.stringify({ gestor: gestorToken, financeiro: financeiroToken, vendedor: vendedorToken }));
  console.log('[global-setup] Tokens salvos em ' + TOKEN_FILE);
}

export default globalSetup;
