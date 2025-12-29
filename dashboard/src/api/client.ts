import axios, { type AxiosInstance, type AxiosResponse } from 'axios'
import type {
  Source,
  Channel,
  Route,
  PublishHistoryItem,
  CreateSourceRequest,
  UpdateSourceRequest,
  CreateChannelRequest,
  UpdateChannelRequest,
  CreateRouteRequest,
  UpdateRouteRequest,
  SourcesListResponse,
  ChannelsListResponse,
  RoutesListResponse,
  PublishHistoryListResponse,
  StatsOverviewResponse,
  StatsPeriod,
} from '../types/publisher'

// Debug mode - logs all requests and responses
const DEBUG = import.meta.env.DEV

// Helper function to get token from localStorage
const getToken = (): string | null => {
  return localStorage.getItem('dashboard_token')
}

// Helper function to handle 401 errors (redirect to login)
const handleUnauthorized = (): void => {
  localStorage.removeItem('dashboard_token')
  // Only redirect if we're in a browser environment
  if (typeof window !== 'undefined') {
    window.location.href = '/dashboard/login'
  }
}

// Create axios instances for each service
const crawlerClient: AxiosInstance = axios.create({
  baseURL: '/api/crawler',
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
  },
})

const sourcesClient: AxiosInstance = axios.create({
  baseURL: '/api/sources',
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
  },
})

const publisherClient: AxiosInstance = axios.create({
  baseURL: '/api/publisher',
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
  },
})

const classifierClient: AxiosInstance = axios.create({
  baseURL: '/api/classifier',
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
  },
})

// Add auth interceptor to all clients
const addAuthInterceptor = (client: AxiosInstance): void => {
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
const addInterceptors = (client: AxiosInstance, serviceName: string): void => {
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
    list: (params?: Record<string, unknown>) => crawlerClient.get('/jobs', { params }),
    get: (id: string | number) => crawlerClient.get(`/jobs/${id}`),
    create: (data: unknown) => crawlerClient.post('/jobs', data),
    update: (id: string | number, data: unknown) => crawlerClient.put(`/jobs/${id}`, data),
    delete: (id: string | number) => crawlerClient.delete(`/jobs/${id}`),
  },

  // Queued Links
  queuedLinks: {
    list: (params?: Record<string, unknown>) => crawlerClient.get('/queued-links', { params }),
    get: (id: string | number) => crawlerClient.get(`/queued-links/${id}`),
    delete: (id: string | number) => crawlerClient.delete(`/queued-links/${id}`),
    createJob: (id: string | number, data: unknown) => crawlerClient.post(`/queued-links/${id}/create-job`, data),
  },

  // Stats
  stats: {
    get: () => crawlerClient.get('/stats'),
  },

  // Articles
  articles: {
    list: (params?: Record<string, unknown>) => crawlerClient.get('/articles', { params }),
  },
}

// Sources API
export const sourcesApi = {
  // Sources CRUD
  list: () => sourcesClient.get(''),
  get: (id: string | number) => sourcesClient.get(`/${id}`),
  create: (data: unknown) => sourcesClient.post('', data),
  update: (id: string | number, data: unknown) => sourcesClient.put(`/${id}`, data),
  delete: (id: string | number) => sourcesClient.delete(`/${id}`),
  fetchMetadata: (url: string) => sourcesClient.post('/fetch-metadata', { url }),

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
    // Note: /stats endpoint doesn't exist, using /stats/overview instead
    get: () => publisherClient.get('/stats/overview?period=all'),
    overview: (period: StatsPeriod = 'today'): Promise<AxiosResponse<StatsOverviewResponse>> =>
      publisherClient.get(`/stats/overview?period=${period}`),
    channels: (since?: string) => publisherClient.get(`/stats/channels${since ? `?since=${since}` : ''}`),
    routes: () => publisherClient.get('/stats/routes'),
  },

  // Recent articles
  articles: {
    recent: (params?: Record<string, unknown>) => publisherClient.get('/articles/recent', { params }),
  },

  // Sources CRUD
  sources: {
    list: (enabledOnly = false): Promise<AxiosResponse<SourcesListResponse>> =>
      publisherClient.get(`/sources${enabledOnly ? '?enabled_only=true' : ''}`),
    get: (id: number): Promise<AxiosResponse<{ source: Source }>> =>
      publisherClient.get(`/sources/${id}`),
    create: (data: CreateSourceRequest): Promise<AxiosResponse<{ source: Source }>> =>
      publisherClient.post('/sources', data),
    update: (id: number, data: UpdateSourceRequest): Promise<AxiosResponse<{ source: Source }>> =>
      publisherClient.put(`/sources/${id}`, data),
    delete: (id: number): Promise<AxiosResponse<void>> =>
      publisherClient.delete(`/sources/${id}`),
  },

  // Channels CRUD
  channels: {
    list: (enabledOnly = false): Promise<AxiosResponse<ChannelsListResponse>> =>
      publisherClient.get(`/channels${enabledOnly ? '?enabled_only=true' : ''}`),
    get: (id: number): Promise<AxiosResponse<{ channel: Channel }>> =>
      publisherClient.get(`/channels/${id}`),
    create: (data: CreateChannelRequest): Promise<AxiosResponse<{ channel: Channel }>> =>
      publisherClient.post('/channels', data),
    update: (id: number, data: UpdateChannelRequest): Promise<AxiosResponse<{ channel: Channel }>> =>
      publisherClient.put(`/channels/${id}`, data),
    delete: (id: number): Promise<AxiosResponse<void>> =>
      publisherClient.delete(`/channels/${id}`),
  },

  // Routes CRUD
  routes: {
    list: (enabledOnly = false): Promise<AxiosResponse<RoutesListResponse>> =>
      publisherClient.get(`/routes${enabledOnly ? '?enabled_only=true' : ''}`),
    get: (id: number): Promise<AxiosResponse<{ route: Route }>> =>
      publisherClient.get(`/routes/${id}`),
    create: (data: CreateRouteRequest): Promise<AxiosResponse<{ route: Route }>> =>
      publisherClient.post('/routes', data),
    update: (id: number, data: UpdateRouteRequest): Promise<AxiosResponse<{ route: Route }>> =>
      publisherClient.put(`/routes/${id}`, data),
    delete: (id: number): Promise<AxiosResponse<void>> =>
      publisherClient.delete(`/routes/${id}`),
  },

  // Publish History
  history: {
    list: (params?: { limit?: number; offset?: number }): Promise<AxiosResponse<PublishHistoryListResponse>> => {
      const query = new URLSearchParams()
      if (params?.limit) query.append('limit', params.limit.toString())
      if (params?.offset) query.append('offset', params.offset.toString())
      return publisherClient.get(`/publish-history${query.toString() ? `?${query.toString()}` : ''}`)
    },
    getByArticle: (articleId: string): Promise<AxiosResponse<{ history: PublishHistoryItem[] }>> =>
      publisherClient.get(`/publish-history/${articleId}`),
  },
}

// Classifier API
export const classifierApi = {
  // Health check
  getHealth: () => axios.get('/api/health/classifier'),

  // Classification
  classify: {
    single: (data: unknown) => classifierClient.post('/classify', data),
    batch: (data: unknown) => classifierClient.post('/classify/batch', data),
    get: (contentId: string) => classifierClient.get(`/classify/${contentId}`),
  },

  // Rules
  rules: {
    list: () => classifierClient.get('/rules'),
    get: (id: string | number) => classifierClient.get(`/rules/${id}`),
    create: (data: unknown) => classifierClient.post('/rules', data),
    update: (id: string | number, data: unknown) => classifierClient.put(`/rules/${id}`, data),
    delete: (id: string | number) => classifierClient.delete(`/rules/${id}`),
  },

  // Sources
  sources: {
    list: () => classifierClient.get('/sources'),
    get: (name: string) => classifierClient.get(`/sources/${name}`),
    update: (name: string, data: unknown) => classifierClient.put(`/sources/${name}`, data),
    stats: (name: string) => classifierClient.get(`/sources/${name}/stats`),
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

