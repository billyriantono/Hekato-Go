import path from 'node:path'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// Built assets are served by the Go server under /admin/ from the ../web directory.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  base: '/admin/',
  resolve: { alias: { '@': path.resolve(__dirname, './src') } },
  build: { outDir: '../web', emptyOutDir: false },
  server: {
    port: 5173,
    proxy: { '/admin/api': 'http://localhost:8080', '/health': 'http://localhost:8080' },
  },
})
