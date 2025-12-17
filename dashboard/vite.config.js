import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [
    vue(),
    tailwindcss(),
  ],
  server: {
    port: 3002,
    proxy: {
      // Crawler API proxy
      '/api/crawler': {
        target: 'http://localhost:8060',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api\/crawler/, '/api/v1'),
      },
      // Source Manager API proxy
      '/api/sources': {
        target: 'http://localhost:8050',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api\/sources/, '/api/v1'),
      },
      // Crawler health endpoint
      '/api/health/crawler': {
        target: 'http://localhost:8060',
        changeOrigin: true,
        rewrite: () => '/health',
      },
    },
  },
  resolve: {
    alias: {
      '@': '/src',
    },
  },
})
