/**
 * Parse Elasticsearch highlight snippets and extract text
 * @param {Object} highlight - ES highlight object
 * @param {String} field - Field name (e.g., 'title', 'body')
 * @param {Number} maxLength - Maximum length for snippet
 * @returns {String} - HTML string with highlights
 */
export function parseHighlight(highlight, field, maxLength = 200) {
  if (!highlight || !highlight[field]) return ''

  // ES returns array of highlighted snippets
  const snippets = highlight[field]
  if (!Array.isArray(snippets) || snippets.length === 0) return ''

  // Join snippets with ellipsis
  let text = snippets.join(' ... ')

  // Truncate if needed (at word boundary)
  if (text.length > maxLength) {
    text = text.substring(0, maxLength)
    const lastSpace = text.lastIndexOf(' ')
    if (lastSpace > 0) {
      text = text.substring(0, lastSpace) + '...'
    }
  }

  return text
}

/**
 * Sanitize HTML to prevent XSS attacks
 * Only allows <em> tags from Elasticsearch highlights
 * @param {String} html - HTML string
 * @returns {String} - Sanitized HTML
 */
export function sanitizeHighlight(html) {
  if (!html) return ''

  // Create temporary div to parse HTML
  const div = document.createElement('div')
  div.innerHTML = html

  // Remove all tags except <em>
  const allowedTags = ['em']
  const allElements = div.getElementsByTagName('*')

  for (let i = allElements.length - 1; i >= 0; i--) {
    const element = allElements[i]
    if (!allowedTags.includes(element.tagName.toLowerCase())) {
      // Replace tag with its text content
      const textNode = document.createTextNode(element.textContent)
      element.parentNode.replaceChild(textNode, element)
    }
  }

  return div.innerHTML
}

/**
 * Extract plain text from highlight (strip all HTML)
 * @param {String} html - HTML string
 * @returns {String} - Plain text
 */
export function stripHighlightTags(html) {
  if (!html) return ''

  const div = document.createElement('div')
  div.innerHTML = html
  return div.textContent || div.innerText || ''
}

/**
 * Truncate text at word boundary
 * @param {String} text - Text to truncate
 * @param {Number} maxLength - Maximum length
 * @returns {String} - Truncated text
 */
export function truncateText(text, maxLength = 200) {
  if (!text || text.length <= maxLength) return text

  let truncated = text.substring(0, maxLength)
  const lastSpace = truncated.lastIndexOf(' ')

  if (lastSpace > 0) {
    truncated = truncated.substring(0, lastSpace)
  }

  return truncated + '...'
}

export default {
  parseHighlight,
  sanitizeHighlight,
  stripHighlightTags,
  truncateText,
}
