import axios, { type AxiosInstance } from 'axios'

export interface VerificationPerson {
  id: string
  community_id: string
  name: string
  role: string
  data_source: string
  is_current: boolean
  verified: boolean
  created_at: string
  updated_at: string
  role_title?: string
  email?: string
  phone?: string
  source_url?: string
  verification_confidence?: number
  verification_issues?: string
}

export interface VerificationBandOffice {
  id: string
  community_id: string
  data_source: string
  verified: boolean
  created_at: string
  updated_at: string
  address_line1?: string
  address_line2?: string
  city?: string
  province?: string
  postal_code?: string
  phone?: string
  fax?: string
  email?: string
  toll_free?: string
  office_hours?: string
  source_url?: string
  verification_confidence?: number
  verification_issues?: string
}

export type EntityType = 'person' | 'band_office'

export interface PendingItem {
  type: EntityType
  person?: VerificationPerson
  band_office?: VerificationBandOffice
}

export interface VerificationStats {
  pending_people: number
  pending_band_offices: number
  scored_people: number
  scored_band_offices: number
  high_confidence: number
  medium_confidence: number
  low_confidence: number
}

interface ListPendingResponse {
  items: PendingItem[]
  total: number
}

interface BulkActionResponse {
  processed: number
  action: string
}

interface ActionResponse {
  message: string
  id: string
  type: string
}

const verificationClient: AxiosInstance = axios.create({
  timeout: 30000,
  headers: { 'Content-Type': 'application/json' },
})

verificationClient.interceptors.request.use((config) => {
  const token = localStorage.getItem('dashboard_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

verificationClient.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('dashboard_token')
      window.location.href = '/dashboard/login'
    }
    return Promise.reject(error)
  }
)

export const verificationApi = {
  listPending(params: { type?: string; limit?: number; offset?: number } = {}): Promise<ListPendingResponse> {
    return verificationClient.get('/api/verification/pending', { params }).then((r) => r.data as ListPendingResponse)
  },

  getStats(): Promise<VerificationStats> {
    return verificationClient.get('/api/verification/stats').then((r) => r.data as VerificationStats)
  },

  verify(id: string, type: EntityType): Promise<ActionResponse> {
    return verificationClient
      .post(`/api/verification/${id}/verify`, null, { params: { type } })
      .then((r) => r.data as ActionResponse)
  },

  reject(id: string, type: EntityType): Promise<ActionResponse> {
    return verificationClient
      .post(`/api/verification/${id}/reject`, null, { params: { type } })
      .then((r) => r.data as ActionResponse)
  },

  bulkVerify(ids: string[], type: EntityType): Promise<BulkActionResponse> {
    return verificationClient
      .post('/api/verification/bulk-verify', { ids, type })
      .then((r) => r.data as BulkActionResponse)
  },

  bulkReject(ids: string[], type: EntityType): Promise<BulkActionResponse> {
    return verificationClient
      .post('/api/verification/bulk-reject', { ids, type })
      .then((r) => r.data as BulkActionResponse)
  },
}
