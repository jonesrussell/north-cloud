import axios, { type AxiosInstance } from 'axios'

const authClient: AxiosInstance = axios.create({
  baseURL: '/api/auth',
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
  },
})

export const authApi = {
  /**
   * Login with username and password
   * @param username - Username
   * @param password - Password
   * @returns Promise with token
   */
  login: (username: string, password: string) => {
    // The proxy rewrites /api/auth to the auth service, so we just need /api/v1/auth/login
    return authClient.post('/api/v1/auth/login', {
      username,
      password,
    })
  },
}

export default authApi

