import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue(), tailwindcss()],
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://192.168.136.97:8050',
        changeOrigin: true,
      },
      '/health': {
        target: 'http://192.168.136.97:8050',
        changeOrigin: true,
      }
    }
  }
})
