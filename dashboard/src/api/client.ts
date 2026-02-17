import axios, { type AxiosInstance, type AxiosResponse } from 'axios'
import type {
  Channel,
  CreateChannelRequest,
  UpdateChannelRequest,
  ChannelsListResponse,
  ChannelPreviewResponse,
  TopicsResponse,
  IndexesResponse,
  PublishHistoryItem,
  PublishHistoryListResponse,
  StatsOverviewResponse,
  ChannelStatsResponse,
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
import type { ImportExcelResult } from '../types/source'
import type { SyncReport } from '../types/crawler'
import type {
  CrimeAggregation,
  LocationAggregation,
  OverviewAggregation,
  MiningAggregation,
  MLHealthResponse,
  AggregationFilters,
  SourceHealthResponse,
  ClassificationDriftAggregation,
  ClassificationDriftTimeseriesResponse,
  ContentTypeMismatchCount,
  SuspectedMisclassificationResponse,
} from '../types/aggregation'

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
    statusCounts: () => crawlerClient.get('/jobs/status-counts'),
    pause: (id: string | number) => crawlerClient.post(`/jobs/${id}/pause`),
    resume: (id: string | number) => crawlerClient.post(`/jobs/${id}/resume`),
    cancel: (id: string | number) => crawlerClient.post(`/jobs/${id}/cancel`),
    retry: (id: string | number) => crawlerClient.post(`/jobs/${id}/retry`),
    forceRun: (id: string | number) => crawlerClient.post(`/v2/jobs/${id}/force-run`),
    // Log endpoints
    logs: (id: string | number, params?: { limit?: number; offset?: number }) =>
      crawlerClient.get(`/jobs/${id}/logs`, { params }),
    viewLogs: (id: string | number, execution?: number | string) => {
      const params = execution !== undefined ? { execution: execution.toString() } : {}
      return crawlerClient.get(`/jobs/${id}/logs/view`, { params })
    },
    downloadLogs: (id: string | number, execution?: number | string) => {
      const params = execution !== undefined ? { execution: execution.toString() } : {}
      return crawlerClient.get(`/jobs/${id}/logs/download`, {
        params,
        responseType: 'blob',
      })
    },
  },
  // Executions
  executions: {
    get: (id: string | number) => crawlerClient.get(`/executions/${id}`),
  },

  // Discovered Links
  discoveredLinks: {
    list: (params?: Record<string, unknown>) => crawlerClient.get('/discovered-links', { params }),
    get: (id: string | number) => crawlerClient.get(`/discovered-links/${id}`),
    delete: (id: string | number) => crawlerClient.delete(`/discovered-links/${id}`),
    createJob: (id: string | number, data: unknown) => crawlerClient.post(`/discovered-links/${id}/create-job`, data),
  },

  // Frontier
  frontier: {
    list: (params?: Record<string, unknown>) => crawlerClient.get('/frontier', { params }),
    stats: () => crawlerClient.get('/frontier/stats'),
    submit: (data: { url: string; source_id: string; origin?: string; priority?: number }) =>
      crawlerClient.post('/frontier/submit', data),
    delete: (id: string) => crawlerClient.delete(`/frontier/${id}`),
  },

  // Stats
  stats: {
    get: () => crawlerClient.get('/stats'),
  },

  // Admin: sync crawl jobs with enabled sources (create missing, resume paused)
  syncEnabledSources: (): Promise<AxiosResponse<SyncReport>> =>
    crawlerClient.post('/admin/sync-enabled-sources'),

  // Articles
  articles: {
    list: (params?: Record<string, unknown>) => crawlerClient.get('/articles', { params }),
  },
}

// Sources API
export const sourcesApi = {
  // Sources CRUD
  list: (params?: {
    limit?: number
    offset?: number
    sort_by?: string
    sort_order?: string
    search?: string
    enabled?: 'true' | 'false'
  }) => sourcesClient.get('', { params }),
  get: (id: string | number) => sourcesClient.get(`/${id}`),
  create: (data: unknown) => sourcesClient.post('', data),
  update: (id: string | number, data: unknown) => sourcesClient.put(`/${id}`, data),
  delete: (id: string | number) => sourcesClient.delete(`/${id}`),
  fetchMetadata: (url: string) => sourcesClient.post('/fetch-metadata', { url }),
  testCrawl: (data: { url: string; selectors?: Record<string, unknown> }) =>
    sourcesClient.post('/test-crawl', data),

  // Import sources from Excel file
  importExcel: (file: File): Promise<AxiosResponse<ImportExcelResult>> => {
    const formData = new FormData()
    formData.append('file', file)
    return sourcesClient.post('/import-excel', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 60000, // Longer timeout for file processing
    })
  },

  // Cities (endpoint is at /api/v1/cities, not under /api/v1/sources)
  cities: {
    list: () => axios.get('/api/cities'),
  },
}

// Publisher API - Routing V2
export const publisherApi = {
  // Health check
  getHealth: (): Promise<AxiosResponse<HealthStatus>> => axios.get('/api/health/publisher'),

  // Stats - shortcut for stats.overview()
  getStats: () => publisherClient.get('/stats/overview?period=all'),

  // Stats
  stats: {
    get: () => publisherClient.get('/stats/overview?period=all'),
    overview: (period: StatsPeriod = 'today'): Promise<AxiosResponse<StatsOverviewResponse>> =>
      publisherClient.get(`/stats/overview?period=${period}`),
    publishVolume: (params?: { hours?: number }): Promise<
      AxiosResponse<{
        hours: number
        messages_total: number
        messages_per_channel: Array<{ channel_name: string; messages_last_24h: number; last_published_at: string | null }>
        generated_at: string
      }>
    > => publisherClient.get('/api/v1/stats/publish-volume', { params }),
    channels: (since?: string): Promise<AxiosResponse<ChannelStatsResponse>> =>
      publisherClient.get(`/stats/channels${since ? `?since=${since}` : ''}`),
    activeChannels: (): Promise<AxiosResponse<ActiveChannelsResponse>> =>
      publisherClient.get('/stats/channels/active'),
  },

  // Recent articles
  articles: {
    recent: (params?: { limit?: number }): Promise<AxiosResponse<RecentArticlesResponse>> =>
      publisherClient.get('/articles/recent', { params }),
  },

  // Topics (Layer 1 - automatic topic channels)
  topics: {
    list: (): Promise<AxiosResponse<TopicsResponse>> => publisherClient.get('/topics'),
  },

  // Indexes (discovered Elasticsearch indexes)
  indexes: {
    list: (): Promise<AxiosResponse<IndexesResponse>> => publisherClient.get('/indexes'),
  },

  // Channels CRUD (Layer 2 - custom channels with rules)
  channels: {
    list: (enabledOnly = false): Promise<AxiosResponse<ChannelsListResponse>> =>
      publisherClient.get(`/channels${enabledOnly ? '?enabled_only=true' : ''}`),
    get: (id: string): Promise<AxiosResponse<Channel>> => publisherClient.get(`/channels/${id}`),
    create: (data: CreateChannelRequest): Promise<AxiosResponse<Channel>> =>
      publisherClient.post('/channels', data),
    update: (id: string, data: UpdateChannelRequest): Promise<AxiosResponse<Channel>> =>
      publisherClient.put(`/channels/${id}`, data),
    delete: (id: string): Promise<AxiosResponse<void>> => publisherClient.delete(`/channels/${id}`),
    preview: (id: string): Promise<AxiosResponse<ChannelPreviewResponse>> =>
      publisherClient.get(`/channels/${id}/preview`),
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
    reclassify: (contentId: string) =>
      classifierClient.post(`/classify/reclassify/${contentId}`),
  },

  // Rules
  rules: {
    list: () => classifierClient.get('/rules'),
    get: (id: string | number) => classifierClient.get(`/rules/${id}`),
    create: (data: unknown) => classifierClient.post('/rules', data),
    update: (id: string | number, data: unknown) => classifierClient.put(`/rules/${id}`, data),
    delete: (id: string | number) => classifierClient.delete(`/rules/${id}`),
    test: (id: string | number, data: { title?: string; body: string }) =>
      classifierClient.post(`/rules/${id}/test`, data),
  },

  // Sources (reputation)
  sources: {
    list: (params?: {
      page?: number
      page_size?: number
      sort_by?: string
      sort_order?: string
      search?: string
      category?: string
    }) => classifierClient.get('/sources', { params }),
    get: (name: string) => classifierClient.get(`/sources/${name}`),
    update: (name: string, data: unknown) => classifierClient.put(`/sources/${name}`, data),
    stats: (name: string) => classifierClient.get(`/sources/${name}/stats`),
  },

  // Statistics
  stats: {
    get: (params?: { date?: string }) => {
      const queryParams = params?.date ? { date: params.date } : {}
      return classifierClient.get('/stats', { params: queryParams })
    },
    topics: () => classifierClient.get('/stats/topics'),
    sources: () => classifierClient.get('/stats/sources'),
  },

  // Metrics
  metrics: {
    mlHealth: (): Promise<AxiosResponse<MLHealthResponse>> =>
      classifierClient.get('/metrics/ml-health'),
  },
}

// Helper to build aggregation query params
const buildAggregationParams = (filters?: AggregationFilters): Record<string, string | string[]> => {
  if (!filters) return {}
  const params: Record<string, string | string[]> = {}
  if (filters.crime_relevance?.length) params.crime_relevance = filters.crime_relevance
  if (filters.crime_sub_labels?.length) params.crime_sub_labels = filters.crime_sub_labels
  if (filters.crime_types?.length) params.crime_types = filters.crime_types
  if (filters.cities?.length) params.cities = filters.cities
  if (filters.provinces?.length) params.provinces = filters.provinces
  if (filters.countries?.length) params.countries = filters.countries
  if (filters.sources?.length) params.sources = filters.sources
  if (filters.min_quality !== undefined) params.min_quality = String(filters.min_quality)
  return params
}

// Index Manager API
export const indexManagerApi = {
  // Health check
  getHealth: (): Promise<AxiosResponse<HealthStatus>> => axios.get('/api/health/index-manager'),

  // Index operations
  indexes: {
    list: (params?: {
      limit?: number
      offset?: number
      sortBy?: string
      sortOrder?: 'asc' | 'desc'
      search?: string
      type?: string
      health?: string
      source?: string
    }): Promise<AxiosResponse<ListIndexesResponse>> =>
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
        if (params.filters.review_required !== undefined) {
          flatParams.review_required = params.filters.review_required
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

  // Aggregations
  aggregations: {
    getCrime: (filters?: AggregationFilters): Promise<AxiosResponse<CrimeAggregation>> => {
      const params = buildAggregationParams(filters)
      return indexManagerClient.get('/api/v1/aggregations/crime', { params })
    },
    getLocation: (filters?: AggregationFilters): Promise<AxiosResponse<LocationAggregation>> => {
      const params = buildAggregationParams(filters)
      return indexManagerClient.get('/api/v1/aggregations/location', { params })
    },
    getOverview: (filters?: AggregationFilters): Promise<AxiosResponse<OverviewAggregation>> => {
      const params = buildAggregationParams(filters)
      return indexManagerClient.get('/api/v1/aggregations/overview', { params })
    },
    getMining: (filters?: AggregationFilters): Promise<AxiosResponse<MiningAggregation>> => {
      const params = buildAggregationParams(filters)
      return indexManagerClient.get('/api/v1/aggregations/mining', { params })
    },
    getSourceHealth: (): Promise<AxiosResponse<SourceHealthResponse>> =>
      indexManagerClient.get('/api/v1/aggregations/source-health'),
    getClassificationDrift: (params?: {
      hours?: number
      sources?: string[]
    }): Promise<AxiosResponse<ClassificationDriftAggregation>> =>
      indexManagerClient.get('/api/v1/aggregations/classification-drift', { params }),
    getClassificationDriftTimeseries: (params?: {
      days?: number
    }): Promise<AxiosResponse<ClassificationDriftTimeseriesResponse>> =>
      indexManagerClient.get('/api/v1/aggregations/classification-drift-timeseries', { params }),
    getContentTypeMismatch: (params?: {
      hours?: number
    }): Promise<AxiosResponse<ContentTypeMismatchCount>> =>
      indexManagerClient.get('/api/v1/aggregations/content-type-mismatch', { params }),
    getSuspectedMisclassifications: (params?: {
      hours?: number
    }): Promise<AxiosResponse<SuspectedMisclassificationResponse>> =>
      indexManagerClient.get('/api/v1/aggregations/suspected-misclassifications', { params }),
  },
}

export default {
  crawler: crawlerApi,
  sources: sourcesApi,
  publisher: publisherApi,
  classifier: classifierApi,
  indexManager: indexManagerApi,
}

