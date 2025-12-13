import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue(), tailwindcss()],
  server: {
    host: '0.0.0.0', // Allow access from outside container
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8050',
        changeOrigin: true,
      },
      '/health': {
        target: 'http://localhost:8050',
        changeOrigin: true,
      }
    }
  }
})
