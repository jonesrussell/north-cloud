import { computed, ref } from 'vue'
import { apiClient } from '../api/client'
import { endpoints } from '../api/endpoints'

const TOKEN_KEY = 'dashboard_token'
const token = ref(localStorage.getItem(TOKEN_KEY))

function isTokenExpired(t: string): boolean {
  try {
    const payload = JSON.parse(atob(t.split('.')[1]))
    return payload.exp * 1000 < Date.now()
  } catch {
    return true
  }
}

export function useAuth() {
  const isAuthenticated = computed(() => {
    if (!token.value) return false
    return !isTokenExpired(token.value)
  })

  async function login(username: string, password: string): Promise<void> {
    const response = await apiClient.post(endpoints.auth.login, { username, password })
    token.value = response.data.token
    localStorage.setItem(TOKEN_KEY, response.data.token)
  }

  function logout(): void {
    token.value = null
    localStorage.removeItem(TOKEN_KEY)
  }

  return { isAuthenticated, token, login, logout }
}
