import path from 'node:path'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// `npm run dev` proxies the API to a running `riven-engine serve`.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: { alias: { '@': path.resolve(__dirname, './src') } },
  server: { proxy: { '/api': 'http://127.0.0.1:7600' } },
  // Relative asset URLs, resolved against <base href> in index.html, so the
  // same build works at / (riven-engine serve) and under /riven (Laravel).
  base: './',
  build: { outDir: 'dist', emptyOutDir: true },
})
