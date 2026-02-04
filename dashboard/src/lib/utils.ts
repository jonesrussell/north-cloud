import { type ClassValue, clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

/**
 * Format a date string for display using the browser's locale.
 * Returns 'N/A' if the date is null, undefined, or empty.
 */
export function formatDate(date: string | null | undefined): string {
  if (!date) return 'N/A'
  return new Date(date).toLocaleString()
}

/**
 * Format a date string showing only the date portion (no time).
 * Returns 'N/A' if the date is null, undefined, or empty.
 */
export function formatDateShort(date: string | null | undefined): string {
  if (!date) return 'N/A'
  return new Date(date).toLocaleDateString()
}
