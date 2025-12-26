import axios from 'axios'
import { useAuth } from '../composables/useAuth'

// Debug mode - logs all requests and responses
const DEBUG = import.meta.env.DEV

// Helper to get auth token
const getAuthToken = () => {
  const { getToken } = useAuth()
  return getToken()
}

// Helper to refresh token on 401
let isRefreshing = false
let failedQueue = []

const processQueue = (error, token = null) => {
  failedQueue.forEach((prom) => {
    if (error) {
      prom.reject(error)
    } else {
      prom.resolve(token)
    }
  })
  failedQueue = []
}

// Add auth interceptor to axios clients
const addAuthInterceptor = (client) => {
  // Request interceptor - add auth token
  client.interceptors.request.use(
    (config) => {
      const token = getAuthToken()
      if (token) {
        config.headers.Authorization = `Bearer ${token}`
      }
      return config
    },
    (error) => {
      return Promise.reject(error)
    }
  )

  // Response interceptor - handle 401 and refresh token
  client.interceptors.response.use(
    (response) => response,
    async (error) => {
      const originalRequest = error.config

      // If error is 401 and we haven't tried to refresh yet
      if (error.response?.status === 401 && !originalRequest._retry) {
        if (isRefreshing) {
          // If already refreshing, queue this request
          return new Promise((resolve, reject) => {
            failedQueue.push({ resolve, reject })
          })
            .then((token) => {
              originalRequest.headers.Authorization = `Bearer ${token}`
              return client(originalRequest)
            })
            .catch((err) => {
              return Promise.reject(err)
            })
        }

        originalRequest._retry = true
        isRefreshing = true

        try {
          const { refresh } = useAuth()
          const refreshed = await refresh()

          if (refreshed) {
            const token = getAuthToken()
            processQueue(null, token)
            originalRequest.headers.Authorization = `Bearer ${token}`
            return client(originalRequest)
          } else {
            processQueue(new Error('Token refresh failed'), null)
            // Redirect to login will be handled by router guard
            return Promise.reject(error)
          }
        } catch (refreshError) {
          processQueue(refreshError, null)
          return Promise.reject(refreshError)
        } finally {
          isRefreshing = false
        }
      }

      return Promise.reject(error)
    }
  )
}

// Create axios instances for each service
const crawlerClient = axios.create({
  baseURL: '/api/crawler',
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
  },
})

const sourcesClient = axios.create({
  baseURL: '/api/sources',
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
  },
})

const publisherClient = axios.create({
  baseURL: '/api/publisher',
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
  },
})

const classifierClient = axios.create({
  baseURL: '/api/classifier',
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
  },
})

// Request/response interceptors for debugging
const addInterceptors = (client, serviceName) => {
  if (DEBUG) {
    client.interceptors.request.use(
      (config) => {
        console.log(`[${serviceName}] Request:`, config.method?.toUpperCase(), config.url, config.data || '')
        return config
      },
      (error) => {
        console.error(`[${serviceName}] Request Error:`, error)
        return Promise.reject(error)
      }
    )

    client.interceptors.response.use(
      (response) => {
        console.log(`[${serviceName}] Response:`, response.status, response.data)
        return response
      },
      (error) => {
        console.error(`[${serviceName}] Response Error:`, error.response?.status, error.response?.data || error.message)
        return Promise.reject(error)
      }
    )
  }
}

// Add auth interceptors to all clients
addAuthInterceptor(crawlerClient)
addAuthInterceptor(sourcesClient)
addAuthInterceptor(publisherClient)
addAuthInterceptor(classifierClient)

// Add debug interceptors
addInterceptors(crawlerClient, 'Crawler')
addInterceptors(sourcesClient, 'Sources')
addInterceptors(publisherClient, 'Publisher')
addInterceptors(classifierClient, 'Classifier')

// Crawler API
export const crawlerApi = {
  // Health check
  getHealth: () => axios.get('/api/health/crawler'),

  // Jobs
  jobs: {
    list: (params) => crawlerClient.get('/jobs', { params }),
    get: (id) => crawlerClient.get(`/jobs/${id}`),
    create: (data) => crawlerClient.post('/jobs', data),
    update: (id, data) => crawlerClient.put(`/jobs/${id}`, data),
    delete: (id) => crawlerClient.delete(`/jobs/${id}`),
  },

  // Stats
  stats: {
    get: () => crawlerClient.get('/stats'),
  },

  // Articles
  articles: {
    list: (params) => crawlerClient.get('/articles', { params }),
  },
}

// Sources API
export const sourcesApi = {
  // Sources CRUD
  list: () => sourcesClient.get(''),
  get: (id) => sourcesClient.get(`/${id}`),
  create: (data) => sourcesClient.post('', data),
  update: (id, data) => sourcesClient.put(`/${id}`, data),
  delete: (id) => sourcesClient.delete(`/${id}`),
  fetchMetadata: (url) => sourcesClient.post('/fetch-metadata', { url }),

  // Cities (endpoint is at /api/v1/cities, not under /api/v1/sources)
  cities: {
    list: () => axios.get('/api/cities'),
  },
}

// Publisher API
export const publisherApi = {
  // Health check
  getHealth: () => axios.get('/api/health/publisher'),

  // Stats
  stats: {
    get: () => publisherClient.get('/stats'),
  },

  // Recent articles
  articles: {
    recent: (params) => publisherClient.get('/articles/recent', { params }),
  },
}

// Classifier API
export const classifierApi = {
  // Health check
  getHealth: () => axios.get('/api/health/classifier'),

  // Classification
  classify: {
    single: (data) => classifierClient.post('/classify', data),
    batch: (data) => classifierClient.post('/classify/batch', data),
    get: (contentId) => classifierClient.get(`/classify/${contentId}`),
  },

  // Rules
  rules: {
    list: () => classifierClient.get('/rules'),
    get: (id) => classifierClient.get(`/rules/${id}`),
    create: (data) => classifierClient.post('/rules', data),
    update: (id, data) => classifierClient.put(`/rules/${id}`, data),
    delete: (id) => classifierClient.delete(`/rules/${id}`),
  },

  // Sources
  sources: {
    list: () => classifierClient.get('/sources'),
    get: (name) => classifierClient.get(`/sources/${name}`),
    update: (name, data) => classifierClient.put(`/sources/${name}`, data),
    stats: (name) => classifierClient.get(`/sources/${name}/stats`),
  },

  // Statistics
  stats: {
    get: () => classifierClient.get('/stats'),
    topics: () => classifierClient.get('/stats/topics'),
    sources: () => classifierClient.get('/stats/sources'),
  },
}

export default {
  crawler: crawlerApi,
  sources: sourcesApi,
  publisher: publisherApi,
  classifier: classifierApi,
}
