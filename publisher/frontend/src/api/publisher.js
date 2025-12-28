import apiClient from './client'

// Sources API
export const sourcesAPI = {
  list: (enabledOnly = false) =>
    apiClient.get(`/sources${enabledOnly ? '?enabled_only=true' : ''}`),

  get: (id) =>
    apiClient.get(`/sources/${id}`),

  create: (data) =>
    apiClient.post('/sources', data),

  update: (id, data) =>
    apiClient.put(`/sources/${id}`, data),

  delete: (id) =>
    apiClient.delete(`/sources/${id}`)
}

// Channels API
export const channelsAPI = {
  list: (enabledOnly = false) =>
    apiClient.get(`/channels${enabledOnly ? '?enabled_only=true' : ''}`),

  get: (id) =>
    apiClient.get(`/channels/${id}`),

  create: (data) =>
    apiClient.post('/channels', data),

  update: (id, data) =>
    apiClient.put(`/channels/${id}`, data),

  delete: (id) =>
    apiClient.delete(`/channels/${id}`)
}

// Routes API
export const routesAPI = {
  list: (enabledOnly = false) =>
    apiClient.get(`/routes${enabledOnly ? '?enabled_only=true' : ''}`),

  get: (id) =>
    apiClient.get(`/routes/${id}`),

  create: (data) =>
    apiClient.post('/routes', data),

  update: (id, data) =>
    apiClient.put(`/routes/${id}`, data),

  delete: (id) =>
    apiClient.delete(`/routes/${id}`)
}

// Stats API
export const statsAPI = {
  overview: (period = 'today') =>
    apiClient.get(`/stats/overview?period=${period}`),

  channels: (since = null) =>
    apiClient.get(`/stats/channels${since ? `?since=${since}` : ''}`),

  routes: () =>
    apiClient.get('/stats/routes')
}

// Publish History API
export const historyAPI = {
  list: (params = {}) => {
    const query = new URLSearchParams(params).toString()
    return apiClient.get(`/publish-history${query ? `?${query}` : ''}`)
  },

  getByArticle: (articleId) =>
    apiClient.get(`/publish-history/${articleId}`)
}
