import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import MockAdapter from 'axios-mock-adapter'
import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { createApp, defineComponent, ref } from 'vue'
import { apiClient } from '@/shared/api/client'
import { useHomeStats, type HomeStats } from '../composables/useHomeStats'

const store: Record<string, string> = {}
const localStorageMock = {
  getItem: vi.fn((key: string) => store[key] ?? null),
  setItem: vi.fn((key: string, value: string) => { store[key] = value }),
  removeItem: vi.fn((key: string) => { delete store[key] }),
}
Object.defineProperty(globalThis, 'localStorage', { value: localStorageMock, writable: true })

function withSetup(composable: () => HomeStats): { result: HomeStats; unmount: () => void } {
  let stats: HomeStats | null = null
  const TestComponent = defineComponent({
    setup() {
      stats = composable()
      return {}
    },
    render: () => null,
  })

  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
    },
  })
  const app = createApp(TestComponent)
  app.use(VueQueryPlugin, { queryClient })

  const root = document.createElement('div')
  app.mount(root)

  return {
    result: stats as unknown as HomeStats,
    unmount: () => {
      app.unmount()
      queryClient.clear()
    },
  }
}

async function settle() {
  await flushPromises()
  await new Promise((r) => setTimeout(r, 50))
  await flushPromises()
}

describe('useHomeStats', () => {
  let mock: MockAdapter

  beforeEach(() => {
    mock = new MockAdapter(apiClient)
    vi.clearAllMocks()
  })

  afterEach(() => {
    mock.restore()
  })

  it('returns source count from paginated response', async () => {
    mock.onGet('/api/sources').reply(200, { total: 42, data: [] })
    mock.onGet('/api/crawler/jobs').reply(200, { jobs: [], total: 0 })
    mock.onGet(/verification/).reply(200, { pending: 0 })
    mock.onGet('/api/publisher/channels').reply(200, { channels: [], count: 0 })

    const { result, unmount } = withSetup(() => useHomeStats())
    await settle()

    expect(result.sourceCount.value.value).toBe(42)
    unmount()
  })

  it('returns running jobs count from total field', async () => {
    mock.onGet('/api/sources').reply(200, { total: 0, data: [] })
    mock.onGet('/api/crawler/jobs').reply(200, {
      jobs: [{ id: '1', status: 'running' }, { id: '2', status: 'running' }],
      total: 2,
    })
    mock.onGet(/verification/).reply(200, { pending: 0 })
    mock.onGet('/api/publisher/channels').reply(200, { channels: [], count: 0 })

    const { result, unmount } = withSetup(() => useHomeStats())
    await settle()

    expect(result.runningJobs.value.value).toBe(2)
    unmount()
  })

  it('returns channel count from count field', async () => {
    mock.onGet('/api/sources').reply(200, { total: 0, data: [] })
    mock.onGet('/api/crawler/jobs').reply(200, { jobs: [], total: 0 })
    mock.onGet(/verification/).reply(200, { pending: 0 })
    mock.onGet('/api/publisher/channels').reply(200, {
      channels: [{ id: '1', name: 'crime' }, { id: '2', name: 'sports' }, { id: '3', name: 'local' }],
      count: 3,
    })

    const { result, unmount } = withSetup(() => useHomeStats())
    await settle()

    expect(result.channelCount.value.value).toBe(3)
    unmount()
  })

  it('shows N/A for pending review when endpoint fails', async () => {
    mock.onGet('/api/sources').reply(200, { total: 5, data: [] })
    mock.onGet('/api/crawler/jobs').reply(200, { jobs: [], total: 0 })
    mock.onGet(/verification/).reply(500)
    mock.onGet('/api/publisher/channels').reply(200, { channels: [], count: 0 })

    const { result, unmount } = withSetup(() => useHomeStats())
    await settle()

    expect(result.pendingReview.value.value).toBe('N/A')
    expect(result.pendingReview.value.isError).toBe(false)
    unmount()
  })

  it('sets isError when source count fails', async () => {
    mock.onGet('/api/sources').reply(500)
    mock.onGet('/api/crawler/jobs').reply(200, { jobs: [], total: 0 })
    mock.onGet(/verification/).reply(200, { pending: 0 })
    mock.onGet('/api/publisher/channels').reply(200, { channels: [], count: 0 })

    const { result, unmount } = withSetup(() => useHomeStats())
    await settle()

    expect(result.sourceCount.value.isError).toBe(true)
    unmount()
  })
})
