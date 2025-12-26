import { ref, computed } from 'vue'
import axios from 'axios'
import { useRouter } from 'vue-router'

const token = ref(localStorage.getItem('auth_token') || null)
const refreshToken = ref(localStorage.getItem('auth_refresh_token') || null)
const user = ref(JSON.parse(localStorage.getItem('auth_user') || 'null'))
const isAuthenticated = computed(() => !!token.value)

export function useAuth() {
  const router = useRouter()

  const login = async (username, password) => {
    try {
      const response = await axios.post('/api/auth/login', {
        username,
        password,
      })

      if (response.data.token) {
        token.value = response.data.token
        refreshToken.value = response.data.refresh_token || null
        user.value = response.data.user

        // Store in localStorage
        localStorage.setItem('auth_token', token.value)
        if (refreshToken.value) {
          localStorage.setItem('auth_refresh_token', refreshToken.value)
        }
        localStorage.setItem('auth_user', JSON.stringify(user.value))

        return { success: true, user: user.value }
      }

      return { success: false, error: 'Invalid response from server' }
    } catch (error) {
      const errorMessage = error.response?.data?.error || error.message || 'Login failed'
      return { success: false, error: errorMessage }
    }
  }

  const logout = async () => {
    try {
      // Call logout endpoint (optional, tokens are stateless)
      if (token.value) {
        await axios.post('/api/auth/logout', {}, {
          headers: {
            Authorization: `Bearer ${token.value}`,
          },
        })
      }
    } catch (error) {
      // Ignore logout errors - we'll clear local state anyway
      console.warn('Logout API call failed:', error)
    } finally {
      // Clear local state
      token.value = null
      refreshToken.value = null
      user.value = null

      localStorage.removeItem('auth_token')
      localStorage.removeItem('auth_refresh_token')
      localStorage.removeItem('auth_user')

      // Redirect to login
      router.push('/login')
    }
  }

  const validate = async () => {
    if (!token.value) {
      return false
    }

    try {
      const response = await axios.get('/api/auth/validate', {
        headers: {
          Authorization: `Bearer ${token.value}`,
        },
      })

      if (response.data.valid && response.data.user) {
        user.value = response.data.user
        localStorage.setItem('auth_user', JSON.stringify(user.value))
        return true
      }

      // Token invalid, clear state
      logout()
      return false
    } catch (error) {
      // Token invalid or expired, try refresh
      if (refreshToken.value) {
        return await refresh()
      }

      // No refresh token, logout
      logout()
      return false
    }
  }

  const refresh = async () => {
    if (!refreshToken.value) {
      return false
    }

    try {
      const response = await axios.post('/api/auth/refresh', {
        refresh_token: refreshToken.value,
      })

      if (response.data.token) {
        token.value = response.data.token
        user.value = response.data.user

        localStorage.setItem('auth_token', token.value)
        localStorage.setItem('auth_user', JSON.stringify(user.value))

        return true
      }

      return false
    } catch (error) {
      // Refresh failed, logout
      logout()
      return false
    }
  }

  const getToken = () => token.value

  return {
    token,
    refreshToken,
    user,
    isAuthenticated,
    login,
    logout,
    validate,
    refresh,
    getToken,
  }
}

