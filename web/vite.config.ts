import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
// Nota: 'test' foi removido — vitest/jsdom não é dependência instalada.
// Testes unitários de componentes podem ser adicionados com `npm install -D vitest @vitest/ui jsdom`.
export default defineConfig({
  plugins: [react()],
})
