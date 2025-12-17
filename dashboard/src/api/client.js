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
  fetchMetadata: (url) => sourcesClient.post('/sources/fetch-metadata', { url }),

  // Cities
  cities: {
    list: () => sourcesClient.get('/cities'),
  },
}

export default {
  crawler: crawlerApi,
  sources: sourcesApi,
}
