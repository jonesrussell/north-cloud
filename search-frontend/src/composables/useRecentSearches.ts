const RECENT_KEY = 'north-cloud-search-recent'
const RECENT_MAX = 10

/**
 * Load recent search queries from localStorage (newest first)
 */
export function getRecentSearches(): string[] {
  try {
    const raw = localStorage.getItem(RECENT_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw) as unknown
    if (!Array.isArray(parsed)) return []
    return parsed.filter((x): x is string => typeof x === 'string').slice(0, RECENT_MAX)
  } catch {
    return []
  }
}

/**
 * Append a query to recent searches (dedupe, move to front, cap at RECENT_MAX)
 */
export function addRecentSearch(query: string): void {
  const trimmed = query.trim()
  if (!trimmed) return
  const recent = getRecentSearches()
  const next = [trimmed, ...recent.filter((q) => q !== trimmed)].slice(0, RECENT_MAX)
  try {
    localStorage.setItem(RECENT_KEY, JSON.stringify(next))
  } catch {
    // ignore
  }
}

export default { getRecentSearches, addRecentSearch }
