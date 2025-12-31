import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { readFileSync } from 'fs'
import { resolve } from 'path'

// API targets - use Docker service names when running in container
const CRAWLER_API_URL = process.env.CRAWLER_API_URL || 'http://localhost:8060'
const SOURCES_API_URL = process.env.SOURCES_API_URL || 'http://localhost:8050'
const PUBLISHER_API_URL = process.env.PUBLISHER_API_URL || 'http://localhost:8070'
const CLASSIFIER_API_URL = process.env.CLASSIFIER_API_URL || 'http://localhost:8071'
// Auth service: use Docker service name when in container, localhost when running locally
const AUTH_API_URL = process.env.AUTH_API_URL || 'http://localhost:8040'

export default defineConfig({
  base: '/dashboard/',
  appType: 'spa',
  plugins: [
    vue(),
    tailwindcss(),
    {
      name: 'spa-fallback',
      configureServer(server) {
        return () => {
          server.middlewares.use((req, res, next) => {
            // Skip API routes, Vite internal routes, and files with extensions
            if (
              req.url?.startsWith('/api/') ||
              req.url?.startsWith('/@') ||
              req.url?.startsWith('/node_modules/') ||
              req.url?.startsWith('/src/') ||
              req.url?.match(/\.[a-zA-Z0-9]+$/)
            ) {
              return next()
            }
            // For routes under /dashboard/, serve index.html
            if (req.url?.startsWith('/dashboard/')) {
              try {
                const htmlPath = resolve(__dirname, 'index.html')
                const html = readFileSync(htmlPath, 'utf-8')
                res.setHeader('Content-Type', 'text/html')
                res.end(html)
                return
              } catch {
                return next()
              }
            }
            next()
          })
        }
      },
    },
  ],
  server: {
    port: 3002,
    // Custom middleware for SPA fallback with base path
    fs: {
      strict: false,
    },
    proxy: {
      // Crawler API proxy
      '/api/crawler': {
        target: CRAWLER_API_URL,
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api\/crawler/, '/api/v1'),
        configure: (proxy, _options) => {
          proxy.on('proxyReq', (proxyReq, req, _res) => {
            // Forward Authorization header if present
            if (req.headers.authorization) {
              proxyReq.setHeader('Authorization', req.headers.authorization)
            }
          })
        },
      },
      // Source Manager API proxy
      '/api/sources': {
        target: SOURCES_API_URL,
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api\/sources/, '/api/v1/sources'),
        configure: (proxy, _options) => {
          proxy.on('proxyReq', (proxyReq, req, _res) => {
            // Forward Authorization header if present
            if (req.headers.authorization) {
              proxyReq.setHeader('Authorization', req.headers.authorization)
            }
          })
        },
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
      // Publisher health endpoint (must come before general /api/publisher route)
      '/api/publisher/health': {
        target: PUBLISHER_API_URL,
        changeOrigin: true,
        rewrite: () => '/health',
      },
      // Publisher API proxy
      '/api/publisher': {
        target: PUBLISHER_API_URL,
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api\/publisher/, '/api/v1'),
        configure: (proxy, _options) => {
          proxy.on('proxyReq', (proxyReq, req) => {
            // Explicitly forward Authorization header
            const authHeader = req.headers.authorization
            if (authHeader) {
              proxyReq.setHeader('Authorization', authHeader)
            }
          })
        },
      },
      // Publisher health endpoint (alternative path)
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
        configure: (proxy, _options) => {
          proxy.on('proxyReq', (proxyReq, req, _res) => {
            // Forward Authorization header if present
            if (req.headers.authorization) {
              proxyReq.setHeader('Authorization', req.headers.authorization)
            }
          })
        },
      },
      // Classifier health endpoint
      '/api/health/classifier': {
        target: CLASSIFIER_API_URL,
        changeOrigin: true,
        rewrite: () => '/health',
      },
      // Auth API proxy - /api/v1/auth route (matches nginx production config)
      '/api/v1/auth': {
        target: AUTH_API_URL,
        changeOrigin: true,
        rewrite: (path) => path, // Pass path as-is since auth service expects /api/v1/auth/login
      },
      // Auth API proxy - legacy /api/auth path (strips prefix like nginx)
      '/api/auth': {
        target: AUTH_API_URL,
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api\/auth/, ''),
      },
    },
  },
  resolve: {
    alias: {
      '@': '/src',
    },
  },
})

