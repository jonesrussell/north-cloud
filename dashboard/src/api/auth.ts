import axios, { type AxiosInstance } from 'axios'

const authClient: AxiosInstance = axios.create({
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
    // Call the auth service endpoint directly - nginx routes /api/v1/auth to auth service
    return authClient.post('/api/v1/auth/login', {
      username,
      password,
    })
  },
}

export default authApi

