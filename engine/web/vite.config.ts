import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// `npm run dev` proxies the API to a running `riven-engine serve`.
export default defineConfig({
  plugins: [react()],
  server: { proxy: { '/api': 'http://127.0.0.1:7600' } },
  build: { outDir: 'dist', emptyOutDir: true },
})
