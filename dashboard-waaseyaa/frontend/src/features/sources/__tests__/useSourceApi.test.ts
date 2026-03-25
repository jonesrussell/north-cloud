import { describe, it, expect, vi, beforeEach } from 'vitest'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'
import { mount, flushPromises } from '@vue/test-utils'
import MockAdapter from 'axios-mock-adapter'
import { apiClient } from '@/shared/api/client'
import { endpoints } from '@/shared/api/endpoints'
import type { Source } from '../types'

// localStorage mock for happy-dom
const store: Record<string, string> = {}
const localStorageMock = {
  getItem: vi.fn((key: string) => store[key] ?? null),
  setItem: vi.fn((key: string, value: string) => {
    store[key] = value
  }),
  removeItem: vi.fn((key: string) => {
    delete store[key]
  }),
}
Object.defineProperty(globalThis, 'localStorage', { value: localStorageMock, writable: true })

const mock = new MockAdapter(apiClient)

function makeSource(overrides: Partial<Source> = {}): Source {
  return {
    id: 'src-1',
    name: 'Test Source',
    url: 'https://example.com',
    rate_limit: '10',
    max_depth: 3,
    time: [],
    selectors: {
      article: { container: 'article', title: 'h1', body: 'article > div' },
      list: { container: '', article_cards: '', article_list: '' },
      page: { container: '', title: '', content: '' },
    },
    enabled: true,
    ingestion_mode: 'crawl',
    feed_poll_interval_minutes: 60,
    allow_source_discovery: false,
    render_mode: 'static',
    type: 'news',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

function createTestApp(setup: () => Record<string, unknown>) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: false },
    },
  })

  const Wrapper = {
    setup() {
      const result = setup()
      return { result }
    },
    template: '<div />',
  }

  const wrapper = mount(Wrapper, {
    global: {
      plugins: [[VueQueryPlugin, { queryClient }]],
    },
  })

  return { wrapper, queryClient }
}

beforeEach(() => {
  mock.reset()
})

describe('useSourceList', () => {
  it('fetches sources from GET /api/sources', async () => {
    const sources = [makeSource(), makeSource({ id: 'src-2', name: 'Second' })]
    mock.onGet(endpoints.sources.list).reply(200, {
      data: sources,
      total: 2,
      page: 1,
      per_page: 20,
    })

    const { useSourceList } = await import('../composables/useSourceApi')

    let data: ReturnType<typeof useSourceList>['data']
    createTestApp(() => {
      const query = useSourceList()
      data = query.data
      return { data }
    })

    await flushPromises()

    expect(data!.value).toBeTruthy()
    expect(data!.value!.data).toHaveLength(2)
    expect(data!.value!.total).toBe(2)
  })
})

describe('useCreateSource', () => {
  it('posts to POST /api/sources', async () => {
    const newSource = makeSource({ id: 'src-new' })
    mock.onPost(endpoints.sources.create).reply(201, newSource)

    const { useCreateSource } = await import('../composables/useSourceApi')

    let mutation: ReturnType<typeof useCreateSource>
    createTestApp(() => {
      mutation = useCreateSource()
      return { mutation }
    })

    const result = await mutation!.mutateAsync({
      name: 'Test Source',
      url: 'https://example.com',
      rate_limit: '10',
      max_depth: 3,
      type: 'news',
      enabled: true,
      ingestion_mode: 'crawl',
      feed_poll_interval_minutes: 60,
      render_mode: 'static',
      allow_source_discovery: false,
      selectors: makeSource().selectors,
    })

    expect(result.id).toBe('src-new')
    expect(mock.history.post).toHaveLength(1)
  })
})

describe('useDeleteSource', () => {
  it('calls DELETE /api/sources/:id', async () => {
    mock.onDelete(endpoints.sources.delete('src-1')).reply(204)

    const { useDeleteSource } = await import('../composables/useSourceApi')

    let mutation: ReturnType<typeof useDeleteSource>
    createTestApp(() => {
      mutation = useDeleteSource()
      return { mutation }
    })

    await mutation!.mutateAsync('src-1')

    expect(mock.history.delete).toHaveLength(1)
    expect(mock.history.delete[0].url).toBe(endpoints.sources.delete('src-1'))
  })
})

describe('useToggleSource', () => {
  it('calls PATCH enable when enabling', async () => {
    mock.onPatch(endpoints.sources.enable('src-1')).reply(200, makeSource())

    const { useToggleSource } = await import('../composables/useSourceApi')

    let mutation: ReturnType<typeof useToggleSource>
    createTestApp(() => {
      mutation = useToggleSource()
      return { mutation }
    })

    await mutation!.mutateAsync({ id: 'src-1', enabled: true })

    expect(mock.history.patch).toHaveLength(1)
    expect(mock.history.patch[0].url).toBe(endpoints.sources.enable('src-1'))
  })

  it('calls PATCH disable when disabling', async () => {
    mock.onPatch(endpoints.sources.disable('src-1')).reply(200, makeSource({ enabled: false }))

    const { useToggleSource } = await import('../composables/useSourceApi')

    let mutation: ReturnType<typeof useToggleSource>
    createTestApp(() => {
      mutation = useToggleSource()
      return { mutation }
    })

    await mutation!.mutateAsync({ id: 'src-1', enabled: false })

    expect(mock.history.patch).toHaveLength(1)
    expect(mock.history.patch[0].url).toBe(endpoints.sources.disable('src-1'))
  })
})

describe('useTestCrawl', () => {
  it('posts to POST /api/sources/test-crawl', async () => {
    const crawlResult = {
      articles_found: 5,
      success_rate: 80,
      warnings: [],
      sample_articles: [],
    }
    mock.onPost(endpoints.sources.testCrawl).reply(200, crawlResult)

    const { useTestCrawl } = await import('../composables/useSourceApi')

    let mutation: ReturnType<typeof useTestCrawl>
    createTestApp(() => {
      mutation = useTestCrawl()
      return { mutation }
    })

    const result = await mutation!.mutateAsync({
      url: 'https://example.com',
      selectors: {},
    })

    expect(result.articles_found).toBe(5)
    expect(mock.history.post).toHaveLength(1)
  })
})
