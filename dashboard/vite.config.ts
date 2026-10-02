import path from 'node:path'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// Built assets are served by the Go server under /admin/ from the ../web directory.
// web/ is wholly generated (public/ + bundle), so each build replaces it.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  base: '/admin/',
  resolve: { alias: { '@': path.resolve(__dirname, './src') } },
  build: { outDir: '../web', emptyOutDir: true },
  server: {
    port: 5173,
    proxy: { '/admin/api/': 'http://localhost:8080', '/health': 'http://localhost:8080' },
  },
})
