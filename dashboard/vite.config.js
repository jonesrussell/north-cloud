import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

// API targets - use Docker service names when running in container
const CRAWLER_API_URL = process.env.CRAWLER_API_URL || 'http://localhost:8060'
const SOURCES_API_URL = process.env.SOURCES_API_URL || 'http://localhost:8050'

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
        target: CRAWLER_API_URL,
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api\/crawler/, '/api/v1'),
      },
      // Source Manager API proxy
      '/api/sources': {
        target: SOURCES_API_URL,
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api\/sources/, '/api/v1'),
      },
      // Crawler health endpoint
      '/api/health/crawler': {
        target: CRAWLER_API_URL,
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
