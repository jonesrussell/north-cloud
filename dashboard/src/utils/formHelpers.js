/**
 * Utility functions for form data transformation and error handling
 */

/**
 * Converts comma-separated string to array of trimmed strings
 * @param {string} str - Comma-separated string
 * @returns {string[]} Array of trimmed strings
 */
export function stringToArray(str) {
  if (!str) return []
  return str.split(',').map(s => s.trim()).filter(Boolean)
}

/**
 * Converts array to comma-separated string
 * @param {string[]} arr - Array of strings
 * @returns {string} Comma-separated string
 */
export function arrayToString(arr) {
  if (!Array.isArray(arr) || arr.length === 0) return ''
  return arr.join(', ')
}

/**
 * Extracts error message from API error response
 * @param {Error} err - Error object
 * @param {string} defaultMessage - Default error message
 * @returns {string} Error message
 */
export function extractErrorMessage(err, defaultMessage = 'An error occurred') {
  return err.response?.data?.error || 
         err.response?.data?.details || 
         err.message || 
         defaultMessage
}

/**
 * Normalizes selectors object to ensure all required keys exist
 * @param {Object} selectors - Selectors object
 * @returns {Object} Normalized selectors object
 */
export function normalizeSelectors(selectors) {
  if (!selectors) {
    return { article: {}, list: {}, page: {} }
  }
  return {
    article: selectors.article || {},
    list: selectors.list || {},
    page: selectors.page || {},
  }
}

/**
 * Merges selector objects, preserving existing values
 * @param {Object} existing - Existing selectors
 * @param {Object} incoming - Incoming selectors to merge
 * @returns {Object} Merged selectors
 */
export function mergeSelectors(existing, incoming) {
  if (!incoming) return existing
  
  const result = { ...existing }
  
  if (incoming.article) {
    result.article = { ...result.article, ...incoming.article }
  }
  if (incoming.list) {
    result.list = { ...result.list, ...incoming.list }
  }
  if (incoming.page) {
    result.page = { ...result.page, ...incoming.page }
  }
  
  return result
}

