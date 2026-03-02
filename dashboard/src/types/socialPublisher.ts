// Social Publisher API Types

// ============================================================================
// Content
// ============================================================================

export interface DeliverySummary {
  total: number
  pending: number
  delivered: number
  failed: number
  retrying: number
}

export interface SocialContent {
  id: string
  type: string
  title: string
  summary: string
  url: string
  project: string
  source: string
  published: boolean
  scheduled_at?: string
  created_at: string
  delivery_summary?: DeliverySummary
}

export interface ContentListResponse {
  items: SocialContent[]
  count: number
  total: number
  offset: number
  limit: number
}

// ============================================================================
// Accounts
// ============================================================================

export interface SocialAccount {
  id: string
  name: string
  platform: string
  project: string
  enabled: boolean
  credentials_configured: boolean
  token_expiry?: string
  created_at: string
  updated_at: string
}

export interface CreateAccountRequest {
  name: string
  platform: string
  project: string
  enabled?: boolean
  credentials?: Record<string, unknown>
  token_expiry?: string
}

export interface UpdateAccountRequest {
  name?: string
  platform?: string
  project?: string
  enabled?: boolean
  credentials?: Record<string, unknown>
  token_expiry?: string
}

export interface AccountsListResponse {
  items: SocialAccount[]
  count: number
}

// ============================================================================
// Deliveries
// ============================================================================

export interface Delivery {
  id: string
  content_id: string
  platform: string
  account: string
  status: string
  attempts: number
  max_attempts: number
  error?: string
  platform_id?: string
  platform_url?: string
  delivered_at?: string
  created_at: string
}

// ============================================================================
// Publishing
// ============================================================================

export interface TargetConfig {
  platform: string
  account: string
}

export interface PublishRequest {
  type: string
  title?: string
  body?: string
  summary?: string
  url?: string
  images?: string[]
  tags?: string[]
  project?: string
  targets?: TargetConfig[]
  scheduled_at?: string
  metadata?: Record<string, string>
  source?: string
}

// ============================================================================
// Filters
// ============================================================================

export interface ContentFilters extends Record<string, unknown> {
  status?: string
  type?: string
}

export interface AccountFilters extends Record<string, unknown> {
  platform?: string
}
