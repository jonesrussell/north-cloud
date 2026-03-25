import type { CrawlJob } from './types'

export function formatDate(dateStr: string | undefined): string {
  if (!dateStr) return '-'
  return new Date(dateStr).toLocaleString()
}

/**
 * Format interval for display. interval_minutes from the API is always
 * stored in minutes regardless of interval_type. We convert to the most
 * readable unit.
 */
export function formatInterval(minutes: number | undefined): string {
  if (!minutes) return 'One-time'
  if (minutes >= 1440 && minutes % 1440 === 0) return `${minutes / 1440}d`
  if (minutes >= 60 && minutes % 60 === 0) return `${minutes / 60}h`
  if (minutes >= 60) return `${Math.round(minutes / 60)}h`
  return `${minutes}m`
}

/**
 * Type-safe cast for DataTable row slots. DataTable types rows as
 * Record<string, unknown>[], so slot props require casting.
 */
export function asJob(row: Record<string, unknown>): CrawlJob {
  return row as unknown as CrawlJob
}
