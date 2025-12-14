import axios from 'axios'

const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8060'

console.log('[API Client] Initializing with base URL:', API_BASE_URL)

const client = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
  timeout: 10000, // 10 second timeout
})

// Request interceptor for debugging
client.interceptors.request.use(
  (config) => {
    console.log('[API Request]', config.method.toUpperCase(), config.url, {
      baseURL: config.baseURL,
      fullURL: `${config.baseURL}${config.url}`,
      headers: config.headers,
    })
    return config
  },
  (error) => {
    console.error('[API Request Error]', error)
    return Promise.reject(error)
  }
)

// Response interceptor for debugging
client.interceptors.response.use(
  (response) => {
    console.log('[API Response]', response.status, response.config.url, response.data)
    return response
  },
  (error) => {
    console.error('[API Response Error]', {
      message: error.message,
      code: error.code,
      url: error.config?.url,
      baseURL: error.config?.baseURL,
      status: error.response?.status,
      statusText: error.response?.statusText,
      data: error.response?.data,
    })
    return Promise.reject(error)
  }
)

export const crawlerApi = {
  // Dashboard / Health
  getHealth: () => {
    console.log('[API] Calling getHealth...')
    return client.get('/health').then(res => {
      console.log('[API] getHealth response:', res.data)
      return res.data
    })
  },

  // Crawl Jobs (placeholder - adjust based on actual API)
  listJobs: () => client.get('/api/v1/jobs').then(res => res.data.jobs || []),
  getJob: (id) => client.get(`/api/v1/jobs/${id}`).then(res => res.data),
  createJob: (data) => client.post('/api/v1/jobs', data).then(res => res.data),
  deleteJob: (id) => client.delete(`/api/v1/jobs/${id}`),

  // Statistics (placeholder - adjust based on actual API)
  getStats: () => {
    console.log('[API] Calling getStats...')
    return client.get('/api/v1/stats').then(res => {
      console.log('[API] getStats response:', res.data)
      return res.data
    })
  },

  // Articles (placeholder - adjust based on actual API)
  listArticles: (params) => client.get('/api/v1/articles', { params }).then(res => res.data.articles || []),
}

export default client
