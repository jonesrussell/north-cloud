import { describe, it, expect, beforeEach, vi } from 'vitest'

const store: Record<string, string> = {}
const localStorageMock = {
  getItem: vi.fn((key: string) => store[key] ?? null),
  setItem: vi.fn((key: string, value: string) => { store[key] = value }),
  removeItem: vi.fn((key: string) => { delete store[key] }),
}
Object.defineProperty(globalThis, 'localStorage', { value: localStorageMock, writable: true })

describe('useAuth', () => {
  beforeEach(() => {
    delete store['dashboard_token']
    vi.clearAllMocks()
    vi.resetModules()
  })

  it('isAuthenticated returns false when no token', async () => {
    const { useAuth } = await import('../useAuth')
    const { isAuthenticated } = useAuth()
    expect(isAuthenticated.value).toBe(false)
  })

  it('isAuthenticated returns true when valid token exists', async () => {
    const payload = btoa(JSON.stringify({ exp: Math.floor(Date.now() / 1000) + 3600 }))
    const fakeJwt = `header.${payload}.signature`
    localStorage.setItem('dashboard_token', fakeJwt)

    const { useAuth } = await import('../useAuth')
    const { isAuthenticated } = useAuth()
    expect(isAuthenticated.value).toBe(true)
  })

  it('logout clears the token', async () => {
    localStorage.setItem('dashboard_token', 'some-token')
    const { useAuth } = await import('../useAuth')
    const { logout } = useAuth()
    logout()
    expect(localStorage.getItem('dashboard_token')).toBeNull()
  })
})
