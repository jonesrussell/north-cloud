// Common types shared across the dashboard

export interface ApiError {
  response?: {
    data?: {
      error?: string
    }
  }
  message?: string
}
