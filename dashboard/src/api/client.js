import axios from 'axios'

// Debug mode - logs all requests and responses
const DEBUG = import.meta.env.DEV

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
  list: () => sourcesClient.get('/sources'),
  get: (id) => sourcesClient.get(`/sources/${id}`),
  create: (data) => sourcesClient.post('/sources', data),
  update: (id, data) => sourcesClient.put(`/sources/${id}`, data),
  delete: (id) => sourcesClient.delete(`/sources/${id}`),
  fetchMetadata: (url) => sourcesClient.post('/fetch-metadata', { url }),

  // Cities
  cities: {
    list: () => sourcesClient.get('/cities'),
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
