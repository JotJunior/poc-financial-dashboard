import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
// Nota: 'test' foi removido — vitest/jsdom não é dependência instalada.
// Testes unitários de componentes podem ser adicionados com `npm install -D vitest @vitest/ui jsdom`.
export default defineConfig({
  plugins: [react()],
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
