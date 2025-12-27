import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

// Search API URL - use Docker service name in container
const SEARCH_API_URL = process.env.SEARCH_API_URL || 'http://localhost:8092'

export default defineConfig({
  plugins: [
    vue(),
    tailwindcss(),
  ],
  server: {
    port: 3003,
    proxy: {
      // Search API proxy
      '/api/search': {
        target: SEARCH_API_URL,
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api\/search/, '/api/v1/search'),
      },
      // Health check endpoint
      '/api/health/search': {
        target: SEARCH_API_URL,
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
