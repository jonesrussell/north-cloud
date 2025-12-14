import axios from 'axios'

const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8060'

const client = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
})

export const crawlerApi = {
  // Dashboard / Health
  getHealth: () => client.get('/health').then(res => res.data),

  // Crawl Jobs (placeholder - adjust based on actual API)
  listJobs: () => client.get('/api/v1/jobs').then(res => res.data.jobs || []),
  getJob: (id) => client.get(`/api/v1/jobs/${id}`).then(res => res.data),
  createJob: (data) => client.post('/api/v1/jobs', data).then(res => res.data),
  deleteJob: (id) => client.delete(`/api/v1/jobs/${id}`),

  // Statistics (placeholder - adjust based on actual API)
  getStats: () => client.get('/api/v1/stats').then(res => res.data),

  // Articles (placeholder - adjust based on actual API)
  listArticles: (params) => client.get('/api/v1/articles', { params }).then(res => res.data.articles || []),
}

export default client
