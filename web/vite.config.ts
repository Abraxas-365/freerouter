import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'path'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(import.meta.dirname, './src'),
    },
  },
  server: {
    // changeOrigin off: the API sees the browser's Host, so the console's
    // requests are same-origin (none/token modes reject other origins).
    proxy: {
      '/api/': { target: 'http://localhost:3000', changeOrigin: false },
      '/v1/': { target: 'http://localhost:3000', changeOrigin: false },
    },
  },
})
