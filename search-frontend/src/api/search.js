import axios from 'axios'

const DEBUG = import.meta.env.DEV

const searchClient = axios.create({
  baseURL: '/api/search',
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
  },
})

// Debug interceptors (development only)
if (DEBUG) {
  searchClient.interceptors.request.use((config) => {
    console.log('[Search API] Request:', config.method?.toUpperCase(), config.url, config.data)
    return config
  })

  searchClient.interceptors.response.use(
    (response) => {
      console.log('[Search API] Response:', response.status, response.data)
      return response
    },
    (error) => {
      console.error('[Search API] Error:', error.response?.status, error.response?.data || error.message)
      return Promise.reject(error)
    }
  )
}

export const searchApi = {
  /**
   * Execute search with filters (complex queries)
   * @param {Object} payload - Search request payload
   * @returns {Promise} - Axios response promise
   */
  search: (payload) => searchClient.post('', payload),

  /**
   * Simple search via query parameters
   * @param {Object} params - Query parameters
   * @returns {Promise} - Axios response promise
   */
  simpleSearch: (params) => searchClient.get('', { params }),

  /**
   * Health check for search service
   * @returns {Promise} - Axios response promise
   */
  health: () => axios.get('/api/health/search'),
}

export default searchApi
