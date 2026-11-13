import path from 'node:path'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// `npm run dev` proxies the API to a running `riven-engine serve`.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: { alias: { '@': path.resolve(__dirname, './src') } },
  server: { proxy: { '/api': 'http://127.0.0.1:7600' } },
  build: { outDir: 'dist', emptyOutDir: true },
})
