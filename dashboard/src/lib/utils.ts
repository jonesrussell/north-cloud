import { type ClassValue, clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

const DISPLAY_TIMEZONE = 'America/New_York'
const DISPLAY_LOCALE = 'en-US'

/**
 * Format a date string for display (date + time) in Eastern Time.
 * Returns 'N/A' if the date is null, undefined, or empty.
 */
export function formatDate(date: string | null | undefined): string {
  if (!date) return 'N/A'
  return new Date(date).toLocaleString(DISPLAY_LOCALE, { timeZone: DISPLAY_TIMEZONE })
}

/**
 * Format a date string showing only the date portion in Eastern Time.
 * Returns 'N/A' if the date is null, undefined, or empty.
 */
export function formatDateShort(date: string | null | undefined): string {
  if (!date) return 'N/A'
  return new Date(date).toLocaleDateString(DISPLAY_LOCALE, { timeZone: DISPLAY_TIMEZONE })
}

/**
 * Format a date string showing only the time portion in Eastern Time.
 * Returns 'N/A' if the date is null, undefined, or empty.
 */
export function formatTime(date: string | null | undefined): string {
  if (!date) return 'N/A'
  return new Date(date).toLocaleTimeString(DISPLAY_LOCALE, { timeZone: DISPLAY_TIMEZONE })
}

/**
 * Format a date as relative time (e.g., "5m ago", "in 2h").
 * Returns '—' if the date is null, undefined, or empty.
 */
export function formatRelativeTime(date: string | null | undefined): string {
  if (!date) return '—'
  const d = new Date(date)
  const now = new Date()
  const diffMs = d.getTime() - now.getTime()
  const absMins = Math.round(Math.abs(diffMs) / 60_000)
  const suffix = diffMs < 0 ? ' ago' : ''
  const prefix = diffMs >= 0 ? 'in ' : ''

  if (absMins < 1) return 'just now'
  if (absMins < 60) return `${prefix}${absMins}m${suffix}`
  const hours = Math.floor(absMins / 60)
  if (hours < 24) return `${prefix}${hours}h${suffix}`
  return `${prefix}${Math.floor(hours / 24)}d${suffix}`
}
