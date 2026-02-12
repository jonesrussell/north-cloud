/**
 * Normalizes a source name for use as an Elasticsearch index prefix.
 * Must match source-manager's sanitizeIndexName (internal/repository/source.go):
 * lowercase, invalid ES chars → _, . and - → _, collapse/trim underscores.
 */

const INVALID_INDEX_CHARS = /[\s"*,/<>?\\|]/g
const CONSECUTIVE_UNDERSCORES = /_{2,}/g

/**
 * Normalizes a source name so it matches the index-manager/ES index prefix.
 * e.g. "ici.radio-canada.ca" → "ici_radio_canada_ca"
 */
export function normalizeSourceNameForIndex(sourceName: string): string {
  if (sourceName === '') {
    return 'unknown'
  }

  let normalized = sourceName.toLowerCase()

  normalized = normalized.replace(INVALID_INDEX_CHARS, '_')
  normalized = normalized.replaceAll('.', '_')
  normalized = normalized.replaceAll('-', '_')
  normalized = normalized.replace(CONSECUTIVE_UNDERSCORES, '_')
  normalized = normalized.trim().replace(/^_+|_+$/g, '')

  if (normalized === '') {
    return 'unknown'
  }

  return normalized
}
