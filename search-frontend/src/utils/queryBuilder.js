/**
 * Build search API payload from form state
 * @param {Object} formData - Form data
 * @returns {Object} - API payload
 */
export function buildSearchPayload(formData) {
  const payload = {
    query: formData.query || '',
    filters: {},
    pagination: {
      page: formData.page || 1,
      size: formData.pageSize || 20,
    },
    sort: {
      field: formData.sortBy || 'relevance',
      order: formData.sortOrder || 'desc',
    },
    options: {
      include_highlights: true,
      include_facets: true,
    },
  }

  // Add filters if present
  if (formData.topics && formData.topics.length > 0) {
    payload.filters.topics = formData.topics
  }
  if (formData.content_type) {
    payload.filters.content_type = formData.content_type
  }
  if (formData.min_quality_score > 0) {
    payload.filters.min_quality_score = formData.min_quality_score
  }
  if (formData.from_date) {
    payload.filters.from_date = formData.from_date
  }
  if (formData.to_date) {
    payload.filters.to_date = formData.to_date
  }
  if (formData.source_names && formData.source_names.length > 0) {
    payload.filters.source_names = formData.source_names
  }

  return payload
}

/**
 * Parse advanced search form to query string
 * @param {Object} formData - Advanced search form data
 * @returns {String} - Query string
 */
export function buildAdvancedQuery(formData) {
  const parts = []

  if (formData.allWords) {
    parts.push(formData.allWords)
  }
  if (formData.exactPhrase) {
    parts.push(`"${formData.exactPhrase}"`)
  }
  if (formData.anyWords) {
    const words = formData.anyWords.split(' ').filter(w => w.trim())
    if (words.length > 0) {
      parts.push(`(${words.join(' OR ')})`)
    }
  }
  if (formData.noneWords) {
    const words = formData.noneWords.split(' ').filter(w => w.trim())
    words.forEach(word => {
      parts.push(`-${word}`)
    })
  }

  return parts.join(' ')
}

/**
 * Validate search form
 * @param {Object} formData - Form data
 * @returns {Object} - Validation result { valid: boolean, errors: Object }
 */
export function validateSearchForm(formData) {
  const errors = {}

  if (!formData.query || formData.query.trim() === '') {
    errors.query = 'Search query is required'
  }

  if (formData.query && formData.query.length > 500) {
    errors.query = 'Query is too long (max 500 characters)'
  }

  if (formData.min_quality_score < 0 || formData.min_quality_score > 100) {
    errors.min_quality_score = 'Quality score must be between 0 and 100'
  }

  if (formData.from_date && formData.to_date) {
    const from = new Date(formData.from_date)
    const to = new Date(formData.to_date)
    if (from > to) {
      errors.date_range = 'Start date must be before end date'
    }
  }

  return {
    valid: Object.keys(errors).length === 0,
    errors,
  }
}

export default {
  buildSearchPayload,
  buildAdvancedQuery,
  validateSearchForm,
}
