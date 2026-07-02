import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

// Build output is embedded by the Go server via go:embed (internal/assets/dist).
export default defineConfig({
  plugins: [vue()],
  // Absolute base: the SPA is served at site root with history-mode routing, so
  // deep-link refreshes (e.g. /nodes) must still resolve /assets/* absolutely.
  base: '/',
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  build: {
    outDir: '../internal/assets/dist',
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/api': { target: 'http://127.0.0.1:8080', changeOrigin: true },
      '/sub': { target: 'http://127.0.0.1:8080', changeOrigin: true },
    },
  },
})
