import axios from 'axios'

const authClient = axios.create({
  baseURL: '/api/auth',
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
  },
})

export const authApi = {
  /**
   * Login with username and password
   * @param {string} username
   * @param {string} password
   * @returns {Promise} - Axios response with token
   */
  login: (username, password) => {
    // The proxy rewrites /api/auth to the auth service, so we just need /api/v1/auth/login
    return authClient.post('/api/v1/auth/login', {
      username,
      password,
    })
  },
}

export default authApi
