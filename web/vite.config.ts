/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
// vitest/jsdom ESTÃO instalados (devDependencies) — a config de testes abaixo
// fornece o ambiente DOM ('jsdom') e os matchers (setup.ts). Sem ela, render()
// falha com "document is not defined".
export default defineConfig({
  plugins: [react()],
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts',
  },
  server: {
    proxy: {
      // Proxia /api/* para o backend Go em desenvolvimento.
      // Alinha com produção (nginx → api:8080) e evita CORS.
      // Para testes E2E headed: Vite em :5173 → Go em :8080.
      '/api': {
        target: process.env.VITE_BACKEND_URL ?? 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
})
