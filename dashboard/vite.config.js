import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

// API targets - use Docker service names when running in container
const CRAWLER_API_URL = process.env.CRAWLER_API_URL || 'http://localhost:8060'
const SOURCES_API_URL = process.env.SOURCES_API_URL || 'http://localhost:8050'
const PUBLISHER_API_URL = process.env.PUBLISHER_API_URL || 'http://localhost:8070'
const CLASSIFIER_API_URL = process.env.CLASSIFIER_API_URL || 'http://localhost:8071'

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
        rewrite: (path) => path.replace(/^\/api\/sources/, '/api/v1/sources'),
      },
      // Source Manager cities endpoint (separate from sources)
      '/api/cities': {
        target: SOURCES_API_URL,
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api\/cities/, '/api/v1/cities'),
      },
      // Crawler health endpoint
      '/api/health/crawler': {
        target: CRAWLER_API_URL,
        changeOrigin: true,
        rewrite: () => '/health',
      },
      // Publisher API proxy
      '/api/publisher': {
        target: PUBLISHER_API_URL,
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api\/publisher/, '/api/v1'),
      },
      // Publisher health endpoint
      '/api/health/publisher': {
        target: PUBLISHER_API_URL,
        changeOrigin: true,
        rewrite: () => '/health',
      },
      // Classifier API proxy
      '/api/classifier': {
        target: CLASSIFIER_API_URL,
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api\/classifier/, '/api/v1'),
      },
      // Classifier health endpoint
      '/api/health/classifier': {
        target: CLASSIFIER_API_URL,
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
