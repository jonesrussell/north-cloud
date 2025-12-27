import axios from 'axios'

// Debug mode - logs all requests and responses
const DEBUG = import.meta.env.DEV

// Helper function to get token from localStorage
const getToken = () => {
  return localStorage.getItem('dashboard_token')
}

// Helper function to handle 401 errors (redirect to login)
const handleUnauthorized = () => {
  localStorage.removeItem('dashboard_token')
  // Only redirect if we're in a browser environment
  if (typeof window !== 'undefined') {
    window.location.href = '/dashboard/login'
  }
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

// Add auth interceptor to all clients
const addAuthInterceptor = (client) => {
  // Request interceptor: Add token to headers
  client.interceptors.request.use(
    (config) => {
      const token = getToken()
      if (token) {
        config.headers.Authorization = `Bearer ${token}`
      }
      return config
    },
    (error) => {
      return Promise.reject(error)
    }
  )

  // Response interceptor: Handle 401 errors
  client.interceptors.response.use(
    (response) => response,
    (error) => {
      if (error.response?.status === 401) {
        handleUnauthorized()
      }
      return Promise.reject(error)
    }
  )
}

// Add auth interceptors to all clients
addAuthInterceptor(crawlerClient)
addAuthInterceptor(sourcesClient)
addAuthInterceptor(publisherClient)
addAuthInterceptor(classifierClient)

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
