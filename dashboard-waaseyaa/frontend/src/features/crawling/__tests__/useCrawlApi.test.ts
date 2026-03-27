import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ref } from 'vue'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'
import { mount, flushPromises } from '@vue/test-utils'
import MockAdapter from 'axios-mock-adapter'
import { apiClient } from '@/shared/api/client'
import { endpoints } from '@/shared/api/endpoints'
import { useCrawlJobs, useCrawlJob, useStartCrawl, useControlJob } from '../composables/useCrawlApi'
import type { CrawlJob, CrawlJobsResponse } from '../types'
import { defineComponent } from 'vue'

// localStorage mock
const store: Record<string, string> = {}
const localStorageMock = {
  getItem: vi.fn((key: string) => store[key] ?? null),
  setItem: vi.fn((key: string, value: string) => { store[key] = value }),
  removeItem: vi.fn((key: string) => { delete store[key] }),
}
Object.defineProperty(globalThis, 'localStorage', { value: localStorageMock, writable: true })

const mockJob: CrawlJob = {
  id: 'job-123',
  source_id: 'source-abc',
  source_name: 'Test Source',
  url: 'https://example.com',
  type: 'crawl',
  status: 'running',
  is_paused: false,
  schedule_enabled: true,
  interval_minutes: 360,
  interval_type: 'minutes',
  max_retries: 3,
  retry_backoff_seconds: 60,
  current_retry_count: 0,
  adaptive_scheduling: true,
  auto_managed: false,
  priority: 0,
  failure_count: 0,
  created_at: '2026-03-24T00:00:00Z',
  updated_at: '2026-03-24T00:00:00Z',
  started_at: '2026-03-24T00:00:00Z',
}

const mockJobsResponse: CrawlJobsResponse = {
  jobs: [mockJob],
  total: 1,
  limit: 50,
  offset: 0,
  sort_by: 'created_at',
  sort_order: 'desc',
}

let mock: MockAdapter

function createTestWrapper(composableFn: () => unknown) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })

  let composableResult: unknown

  const TestComponent = defineComponent({
    setup() {
      composableResult = composableFn()
      return {}
    },
    template: '<div></div>',
  })

  const wrapper = mount(TestComponent, {
    global: {
      plugins: [[VueQueryPlugin, { queryClient }]],
    },
  })

  return { wrapper, getResult: () => composableResult, queryClient }
}

describe('useCrawlApi', () => {
  beforeEach(() => {
    mock = new MockAdapter(apiClient)
  })

  describe('useCrawlJobs', () => {
    it('fetches jobs list', async () => {
      mock.onGet(endpoints.crawler.jobs).reply(200, mockJobsResponse)

      const { getResult } = createTestWrapper(() => useCrawlJobs())
      await flushPromises()

      const result = getResult() as ReturnType<typeof useCrawlJobs>
      expect(result.data.value).toEqual(mockJobsResponse)
    })

    it('handles fetch error', async () => {
      mock.onGet(endpoints.crawler.jobs).reply(500)

      const { getResult } = createTestWrapper(() => useCrawlJobs())
      await flushPromises()

      const result = getResult() as ReturnType<typeof useCrawlJobs>
      expect(result.isError.value).toBe(true)
    })
  })

  describe('useCrawlJob', () => {
    it('fetches a single job by id', async () => {
      const jobId = 'job-123'
      mock.onGet(endpoints.crawler.job(jobId)).reply(200, mockJob)

      const id = ref(jobId)
      const { getResult } = createTestWrapper(() => useCrawlJob(id))
      await flushPromises()

      const result = getResult() as ReturnType<typeof useCrawlJob>
      expect(result.data.value).toEqual(mockJob)
    })

    it('does not fetch when id is empty', async () => {
      const id = ref('')
      const { getResult } = createTestWrapper(() => useCrawlJob(id))
      await flushPromises()

      const result = getResult() as ReturnType<typeof useCrawlJob>
      expect(result.isFetching.value).toBe(false)
    })
  })

  describe('useStartCrawl', () => {
    it('posts a new crawl job', async () => {
      mock.onPost(endpoints.crawler.jobs).reply(201, mockJob)

      const { getResult } = createTestWrapper(() => useStartCrawl())
      const result = getResult() as ReturnType<typeof useStartCrawl>

      result.mutate({
        source_id: 'source-abc',
        url: 'https://example.com',
        schedule_enabled: false,
      })
      await flushPromises()

      expect(result.data.value).toEqual(mockJob)
    })

    it('handles creation error', async () => {
      mock.onPost(endpoints.crawler.jobs).reply(400, { error: 'source_id is required' })

      const { getResult } = createTestWrapper(() => useStartCrawl())
      const result = getResult() as ReturnType<typeof useStartCrawl>

      result.mutate({
        source_id: '',
        url: 'https://example.com',
        schedule_enabled: false,
      })
      await flushPromises()

      expect(result.isError.value).toBe(true)
    })
  })

  describe('useControlJob', () => {
    it('sends pause action', async () => {
      const pausedJob = { ...mockJob, status: 'paused' as const }
      mock.onPost(`${endpoints.crawler.job('job-123')}/pause`).reply(200, pausedJob)

      const { getResult } = createTestWrapper(() => useControlJob())
      const result = getResult() as ReturnType<typeof useControlJob>

      result.mutate({ id: 'job-123', action: 'pause' })
      await flushPromises()

      expect(result.data.value?.status).toBe('paused')
    })

    it('sends resume action', async () => {
      const resumedJob = { ...mockJob, status: 'scheduled' as const }
      mock.onPost(`${endpoints.crawler.job('job-123')}/resume`).reply(200, resumedJob)

      const { getResult } = createTestWrapper(() => useControlJob())
      const result = getResult() as ReturnType<typeof useControlJob>

      result.mutate({ id: 'job-123', action: 'resume' })
      await flushPromises()

      expect(result.data.value?.status).toBe('scheduled')
    })

    it('sends cancel action', async () => {
      const cancelledJob = { ...mockJob, status: 'cancelled' as const }
      mock.onPost(`${endpoints.crawler.job('job-123')}/cancel`).reply(200, cancelledJob)

      const { getResult } = createTestWrapper(() => useControlJob())
      const result = getResult() as ReturnType<typeof useControlJob>

      result.mutate({ id: 'job-123', action: 'cancel' })
      await flushPromises()

      expect(result.data.value?.status).toBe('cancelled')
    })

    it('handles control error', async () => {
      mock.onPost(`${endpoints.crawler.job('job-123')}/pause`).reply(400, { error: 'Job not found' })

      const { getResult } = createTestWrapper(() => useControlJob())
      const result = getResult() as ReturnType<typeof useControlJob>

      result.mutate({ id: 'job-123', action: 'pause' })
      await flushPromises()

      expect(result.isError.value).toBe(true)
    })
  })
})
