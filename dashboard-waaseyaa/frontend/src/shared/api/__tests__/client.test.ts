import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import MockAdapter from 'axios-mock-adapter'
import { apiClient } from '../client'

const store: Record<string, string> = {}
const localStorageMock = {
  getItem: vi.fn((key: string) => store[key] ?? null),
  setItem: vi.fn((key: string, value: string) => { store[key] = value }),
  removeItem: vi.fn((key: string) => { delete store[key] }),
}
Object.defineProperty(globalThis, 'localStorage', { value: localStorageMock, writable: true })

describe('apiClient', () => {
  let mock: MockAdapter

  beforeEach(() => {
    mock = new MockAdapter(apiClient)
    delete store['dashboard_token']
    vi.clearAllMocks()
  })

  afterEach(() => {
    mock.restore()
  })

  it('attaches Authorization header when token exists', async () => {
    localStorage.setItem('dashboard_token', 'test-jwt-token')
    mock.onGet('/test').reply(200, { ok: true })

    await apiClient.get('/test')
    expect(mock.history.get[0].headers?.Authorization).toBe('Bearer test-jwt-token')
  })

  it('does not attach header when no token', async () => {
    mock.onGet('/test').reply(200, { ok: true })

    await apiClient.get('/test')
    expect(mock.history.get[0].headers?.Authorization).toBeUndefined()
  })
})
