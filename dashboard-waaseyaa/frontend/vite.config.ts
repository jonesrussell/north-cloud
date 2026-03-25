import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { fileURLToPath, URL } from 'node:url'

export default defineConfig({
  plugins: [vue(), tailwindcss()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 3002,
    proxy: {
      '/api/auth': {
        target: 'http://localhost:8040',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api\/auth/, '/api/v1/auth'),
      },
      '/api/sources': {
        target: 'http://localhost:8050',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api\/sources/, '/api/v1/sources'),
      },
      '/api/crawler': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api\/crawler/, '/api/v1/crawler'),
      },
      '/api/publisher': {
        target: 'http://localhost:8070',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api\/publisher/, '/api/v1/publisher'),
      },
      '/api/classifier': {
        target: 'http://localhost:8070',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api\/classifier/, '/api/v1/classifier'),
      },
      '/api/index-manager': {
        target: 'http://localhost:8090',
        changeOrigin: true,
        rewrite: (path) =>
          path.replace(/^\/api\/index-manager/, '/api/v1/index-manager'),
      },
      '/api/search': {
        target: 'http://localhost:8092',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api\/search/, '/api/v1/search'),
      },
    },
  },
  test: {
    environment: 'happy-dom',
  },
})
