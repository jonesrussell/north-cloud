import axios from 'axios'

const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8050'

const client = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
})

export const sourcesApi = {
  list: () => client.get('/api/v1/sources').then(res => res.data.sources || []),
  get: (id) => client.get(`/api/v1/sources/${id}`).then(res => res.data),
  create: (data) => client.post('/api/v1/sources', data).then(res => res.data),
  update: (id, data) => client.put(`/api/v1/sources/${id}`, data).then(res => res.data),
  delete: (id) => client.delete(`/api/v1/sources/${id}`),
}

export const citiesApi = {
  list: () => client.get('/api/v1/cities').then(res => res.data.cities || []),
}

export default client

