import type { Source } from './types'

/** Derive a display status from source fields. */
export function getSourceStatus(source: { disabled_at?: string | null; enabled?: boolean }): string {
  if (source.disabled_at) return 'disabled'
  if (source.enabled) return 'active'
  return 'paused'
}

/** Format an ISO date string for display, or return '-' for nullish values. */
export function formatDate(dateStr: string | null | undefined): string {
  if (!dateStr) return '-'
  return new Date(dateStr).toLocaleDateString('en-CA', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  })
}

/** Format an ISO date string with time for detail views. */
export function formatDateTime(dateStr: string | null | undefined): string {
  if (!dateStr) return '-'
  return new Date(dateStr).toLocaleString('en-CA')
}

/** Extract a human-readable error message from a query error. */
export function getErrorMessage(error: unknown, fallback: string): string {
  if (!error) return fallback
  const err = error as { message?: string }
  return err.message ?? fallback
}
