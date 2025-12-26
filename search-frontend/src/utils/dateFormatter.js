/**
 * Format date string to readable format
 * @param {String} dateString - ISO date string
 * @returns {String} - Formatted date
 */
export function formatDate(dateString) {
  if (!dateString) return ''

  const date = new Date(dateString)
  if (isNaN(date.getTime())) return dateString

  const options = { year: 'numeric', month: 'short', day: 'numeric' }
  return date.toLocaleDateString('en-US', options)
}

/**
 * Format date to relative time (e.g., "2 hours ago")
 * @param {String} dateString - ISO date string
 * @returns {String} - Relative time string
 */
export function formatRelativeTime(dateString) {
  if (!dateString) return ''

  const date = new Date(dateString)
  if (isNaN(date.getTime())) return dateString

  const now = new Date()
  const diffMs = now - date
  const diffMins = Math.floor(diffMs / 60000)
  const diffHours = Math.floor(diffMs / 3600000)
  const diffDays = Math.floor(diffMs / 86400000)

  if (diffMins < 1) return 'just now'
  if (diffMins < 60) return `${diffMins} minute${diffMins > 1 ? 's' : ''} ago`
  if (diffHours < 24) return `${diffHours} hour${diffHours > 1 ? 's' : ''} ago`
  if (diffDays < 7) return `${diffDays} day${diffDays > 1 ? 's' : ''} ago`

  return formatDate(dateString)
}

/**
 * Get current date in YYYY-MM-DD format
 * @returns {String} - Date string
 */
export function getTodayString() {
  const today = new Date()
  return today.toISOString().split('T')[0]
}

/**
 * Get date N days ago in YYYY-MM-DD format
 * @param {Number} days - Number of days
 * @returns {String} - Date string
 */
export function getDaysAgoString(days) {
  const date = new Date()
  date.setDate(date.getDate() - days)
  return date.toISOString().split('T')[0]
}

export default {
  formatDate,
  formatRelativeTime,
  getTodayString,
  getDaysAgoString,
}
