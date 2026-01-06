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
  HealthStatus,
  ActiveChannelsResponse,
  RecentArticlesResponse,
} from '../types/publisher'
import type {
  Index,
  CreateIndexRequest,
  CreateSourceIndexesRequest,
  ListIndexesResponse,
  GetIndexResponse,
  IndexHealthResponse,
  CreateSourceIndexesResponse,
  IndexStats,
  Document,
  DocumentQueryRequest,
  DocumentQueryResponse,
} from '../types/indexManager'

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
  timeout: 30000, // Increased to 30s to handle slow database operations
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

const indexManagerClient: AxiosInstance = axios.create({
  baseURL: '/api/index-manager',
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
addAuthInterceptor(indexManagerClient)

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
addInterceptors(indexManagerClient, 'IndexManager')

// Crawler API
export const crawlerApi = {
  // Health check
  getHealth: () => axios.get('/api/health/crawler'),
  
  // Stats - shortcut for stats.get()
  getStats: () => crawlerClient.get('/stats'),

  // Jobs
  jobs: {
    list: (params?: Record<string, unknown>) => crawlerClient.get('/jobs', { params }),
    get: (id: string | number) => crawlerClient.get(`/jobs/${id}`),
    create: (data: unknown) => crawlerClient.post('/jobs', data),
    update: (id: string | number, data: unknown) => crawlerClient.put(`/jobs/${id}`, data),
    delete: (id: string | number) => crawlerClient.delete(`/jobs/${id}`),
    executions: (id: string | number, params?: { limit?: number; offset?: number }) =>
      crawlerClient.get(`/jobs/${id}/executions`, { params }),
    stats: (id: string | number) => crawlerClient.get(`/jobs/${id}/stats`),
    pause: (id: string | number) => crawlerClient.post(`/jobs/${id}/pause`),
    resume: (id: string | number) => crawlerClient.post(`/jobs/${id}/resume`),
    cancel: (id: string | number) => crawlerClient.post(`/jobs/${id}/cancel`),
    retry: (id: string | number) => crawlerClient.post(`/jobs/${id}/retry`),
  },
  // Executions
  executions: {
    get: (id: string | number) => crawlerClient.get(`/executions/${id}`),
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
  testCrawl: (data: { url: string; selectors?: Record<string, unknown> }) =>
    sourcesClient.post('/test-crawl', data),

  // Cities (endpoint is at /api/v1/cities, not under /api/v1/sources)
  cities: {
    list: () => axios.get('/api/cities'),
  },
}

// Publisher API
export const publisherApi = {
  // Health check
  getHealth: (): Promise<AxiosResponse<HealthStatus>> => publisherClient.get('/health'),
  health: (): Promise<AxiosResponse<HealthStatus>> => publisherClient.get('/health'),
  
  // Stats - shortcut for stats.overview()
  getStats: () => publisherClient.get('/stats/overview?period=all'),

  // Stats
  stats: {
    // Note: /stats endpoint doesn't exist, using /stats/overview instead
    get: () => publisherClient.get('/stats/overview?period=all'),
    overview: (period: StatsPeriod = 'today'): Promise<AxiosResponse<StatsOverviewResponse>> =>
      publisherClient.get(`/stats/overview?period=${period}`),
    channels: (since?: string) => publisherClient.get(`/stats/channels${since ? `?since=${since}` : ''}`),
    activeChannels: (): Promise<AxiosResponse<ActiveChannelsResponse>> =>
      publisherClient.get('/stats/channels/active'),
    routes: () => publisherClient.get('/stats/routes'),
  },

  // Recent articles
  articles: {
    recent: (params?: { limit?: number }): Promise<AxiosResponse<RecentArticlesResponse>> =>
      publisherClient.get('/articles/recent', { params }),
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
    testPublish: (id: number): Promise<AxiosResponse<{
      channel_name: string
      channel_id: string
      routes_count: number
      estimated_count: number
      route_stats: Array<{
        route_id: string
        source_name: string
        min_quality_score: number
        topics: string[]
        estimated_count: number
      }>
      sample_articles: Array<{
        title: string
        quality_score: number
        topics: string[]
        published_date: string
        url: string
        source: string
        route_id: string
      }>
      message: string
    }>> => publisherClient.get(`/channels/${id}/test-publish`),
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
    preview: (params: {
      source_id?: string
      min_quality_score?: string
      topics?: string
    }): Promise<AxiosResponse<{
      estimated_count: number
      filters: {
        source_id?: string
        min_quality_score: string
        topics?: string
      }
      sample_articles: Array<{
        title: string
        quality_score: number
        topics: string[]
        published_date: string
        url: string
      }>
    }>> => {
      const query = new URLSearchParams()
      if (params.source_id) query.append('source_id', params.source_id)
      if (params.min_quality_score) query.append('min_quality_score', params.min_quality_score)
      if (params.topics) query.append('topics', params.topics)
      return publisherClient.get(`/routes/preview${query.toString() ? `?${query.toString()}` : ''}`)
    },
  },

  // Publish History
  history: {
    list: (params?: {
      limit?: number
      offset?: number
      channel_name?: string
      article_id?: string
      start_date?: string
      end_date?: string
    }): Promise<AxiosResponse<PublishHistoryListResponse>> => {
      const query = new URLSearchParams()
      if (params?.limit) query.append('limit', params.limit.toString())
      if (params?.offset) query.append('offset', params.offset.toString())
      if (params?.channel_name) query.append('channel_name', params.channel_name)
      if (params?.article_id) query.append('article_id', params.article_id)
      if (params?.start_date) query.append('start_date', params.start_date)
      if (params?.end_date) query.append('end_date', params.end_date)
      return publisherClient.get(`/publish-history${query.toString() ? `?${query.toString()}` : ''}`)
    },
    getByArticle: (articleId: string): Promise<AxiosResponse<{ history: PublishHistoryItem[] }>> =>
      publisherClient.get(`/publish-history/${articleId}`),
    clearAll: (): Promise<AxiosResponse<{ message: string; deleted: number }>> =>
      publisherClient.delete('/publish-history'),
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

// Index Manager API
export const indexManagerApi = {
  // Health check
  getHealth: (): Promise<AxiosResponse<HealthStatus>> => indexManagerClient.get('/health'),

  // Index operations
  indexes: {
    list: (params?: { type?: string; source?: string }): Promise<AxiosResponse<ListIndexesResponse>> =>
      indexManagerClient.get('/api/v1/indexes', { params }),
    get: (indexName: string): Promise<AxiosResponse<GetIndexResponse>> =>
      indexManagerClient.get(`/api/v1/indexes/${indexName}`),
    create: (data: CreateIndexRequest): Promise<AxiosResponse<{ index: Index }>> =>
      indexManagerClient.post('/api/v1/indexes', data),
    delete: (indexName: string): Promise<AxiosResponse<void>> =>
      indexManagerClient.delete(`/api/v1/indexes/${indexName}`),
    getHealth: (indexName: string): Promise<AxiosResponse<IndexHealthResponse>> =>
      indexManagerClient.get(`/api/v1/indexes/${indexName}/health`),
  },

  // Source operations
  sources: {
    createIndexes: (
      sourceName: string,
      data?: CreateSourceIndexesRequest
    ): Promise<AxiosResponse<CreateSourceIndexesResponse>> =>
      indexManagerClient.post(`/api/v1/sources/${sourceName}/indexes`, data),
    listIndexes: (sourceName: string): Promise<AxiosResponse<ListIndexesResponse>> =>
      indexManagerClient.get(`/api/v1/sources/${sourceName}/indexes`),
    deleteIndexes: (sourceName: string): Promise<AxiosResponse<void>> =>
      indexManagerClient.delete(`/api/v1/sources/${sourceName}/indexes`),
  },

  // Stats
  stats: {
    get: (): Promise<AxiosResponse<IndexStats>> => indexManagerClient.get('/api/v1/stats'),
  },

  // Document operations
  documents: {
    query: (
      indexName: string,
      params?: DocumentQueryRequest
    ): Promise<AxiosResponse<DocumentQueryResponse>> => {
      // Flatten nested params for GET request (backend expects flat query params)
      const flatParams: Record<string, unknown> = {}
      if (params?.query) {
        flatParams.query = params.query
      }
      if (params?.pagination) {
        flatParams.page = params.pagination.page
        flatParams.size = params.pagination.size
      }
      if (params?.sort) {
        flatParams.sort_field = params.sort.field
        flatParams.sort_order = params.sort.order
      }
      if (params?.filters) {
        if (params.filters.is_crime_related !== undefined) {
          flatParams.is_crime_related = params.filters.is_crime_related
        }
        if (params.filters.content_type) {
          flatParams.content_type = params.filters.content_type
        }
        if (params.filters.min_quality_score) {
          flatParams.min_quality_score = params.filters.min_quality_score
        }
        if (params.filters.max_quality_score) {
          flatParams.max_quality_score = params.filters.max_quality_score
        }
        if (params.filters.topics && params.filters.topics.length > 0) {
          flatParams.topics = params.filters.topics.join(',')
        }
      }
      return indexManagerClient.get(`/api/v1/indexes/${indexName}/documents`, { params: flatParams })
    },
    get: (indexName: string, documentId: string): Promise<AxiosResponse<Document>> =>
      indexManagerClient.get(`/api/v1/indexes/${indexName}/documents/${documentId}`),
    update: (
      indexName: string,
      documentId: string,
      data: Document
    ): Promise<AxiosResponse<void>> =>
      indexManagerClient.put(`/api/v1/indexes/${indexName}/documents/${documentId}`, data),
    delete: (indexName: string, documentId: string): Promise<AxiosResponse<void>> =>
      indexManagerClient.delete(`/api/v1/indexes/${indexName}/documents/${documentId}`),
    bulkDelete: (
      indexName: string,
      documentIds: string[]
    ): Promise<AxiosResponse<void>> =>
      indexManagerClient.post(`/api/v1/indexes/${indexName}/documents/bulk-delete`, {
        document_ids: documentIds,
      }),
  },
}

export default {
  crawler: crawlerApi,
  sources: sourcesApi,
  publisher: publisherApi,
  classifier: classifierApi,
  indexManager: indexManagerApi,
}

