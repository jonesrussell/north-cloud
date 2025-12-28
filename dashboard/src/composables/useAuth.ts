import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { authApi } from '../api/auth'

const TOKEN_KEY = 'dashboard_token'

// Reactive state
const token = ref<string | null>(localStorage.getItem(TOKEN_KEY) || null)
const loading = ref(false)
const error = ref<string | null>(null)

export function useAuth() {
  const router = useRouter()

  const isAuthenticated = computed(() => !!token.value)

  /**
   * Login with username and password
   */
  const login = async (username: string, password: string): Promise<{ success: boolean; error?: string }> => {
    loading.value = true
    error.value = null

    try {
      const response = await authApi.login(username, password)
      const { token: newToken } = response.data as { token?: string }

      if (newToken) {
        token.value = newToken
        localStorage.setItem(TOKEN_KEY, newToken)
        return { success: true }
      } else {
        error.value = 'No token received from server'
        return { success: false, error: error.value }
      }
    } catch (err: unknown) {
      const axiosError = err as { response?: { status?: number; data?: { error?: string } }; message?: string }
      if (axiosError.response?.status === 401) {
        error.value = 'Invalid username or password'
      } else {
        error.value = axiosError.response?.data?.error || axiosError.message || 'Login failed'
      }
      return { success: false, error: error.value }
    } finally {
      loading.value = false
    }
  }

  /**
   * Logout and clear token
   */
  const logout = (): void => {
    token.value = null
    localStorage.removeItem(TOKEN_KEY)
    router.push('/login')
  }

  /**
   * Get current token
   */
  const getToken = (): string | null => {
    return token.value
  }

  /**
   * Check if user is authenticated
   */
  const checkAuth = (): boolean => {
    const storedToken = localStorage.getItem(TOKEN_KEY)
    if (storedToken) {
      token.value = storedToken
    }
    return !!token.value
  }

  return {
    token: computed(() => token.value),
    isAuthenticated,
    loading: computed(() => loading.value),
    error: computed(() => error.value),
    login,
    logout,
    getToken,
    checkAuth,
  }
}

