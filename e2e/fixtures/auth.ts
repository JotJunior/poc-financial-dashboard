/**
 * Shared auth fixture — lê tokens do globalSetup em /tmp/e2e-tokens.json.
 * Tokens gerados UMA VEZ pelo global-setup, evitando rate limit de login.
 * Ref: tasks.md §9.2; quickstart.md §7
 */
import { test as base } from '@playwright/test';
import { readFileSync } from 'fs';

const TOKEN_FILE = '/tmp/e2e-tokens.json';

function readTokens(): { gestor: string; financeiro: string; vendedor: string } {
  try {
    const data = readFileSync(TOKEN_FILE, 'utf-8');
    return JSON.parse(data);
  } catch (err) {
    throw new Error(
      `[auth fixture] Tokens não encontrados em ${TOKEN_FILE}. ` +
      `Execute o global-setup primeiro (playwright.config.ts globalSetup). Erro: ${err}`
    );
  }
}

export type AuthFixtures = {
  gestorToken: string;
  financeiroToken: string;
  vendedorToken: string;
};

export const test = base.extend<AuthFixtures>({
  gestorToken: async ({}, use) => {
    const tokens = readTokens();
    await use(tokens.gestor);
  },
  financeiroToken: async ({}, use) => {
    const tokens = readTokens();
    await use(tokens.financeiro);
  },
  vendedorToken: async ({}, use) => {
    const tokens = readTokens();
    await use(tokens.vendedor);
  },
});

export { expect } from '@playwright/test';
