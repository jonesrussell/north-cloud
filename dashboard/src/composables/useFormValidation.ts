import { ref, computed, watch } from 'vue'

export interface ValidationRule {
  required?: boolean
  minLength?: number
  maxLength?: number
  pattern?: RegExp
  url?: boolean
  custom?: (value: any) => string | null
}

export interface ValidationResult {
  isValid: boolean
  error: string | null
}

export interface FieldValidation {
  value: any
  error: string | null
  touched: boolean
  validating: boolean
  isValid: boolean
}

export function useFormValidation() {
  const fields = ref<Record<string, FieldValidation>>({})

  function registerField(name: string, initialValue: any = '') {
    fields.value[name] = {
      value: initialValue,
      error: null,
      touched: false,
      validating: false,
      isValid: true,
    }
  }

  function validateField(name: string, rules: ValidationRule): ValidationResult {
    const field = fields.value[name]
    if (!field) {
      return { isValid: false, error: 'Field not registered' }
    }

    const value = field.value

    // Required validation
    if (rules.required && (!value || value.toString().trim() === '')) {
      return { isValid: false, error: 'This field is required' }
    }

    // If field is empty and not required, skip other validations
    if (!value || value.toString().trim() === '') {
      return { isValid: true, error: null }
    }

    // Min length validation
    if (rules.minLength && value.toString().length < rules.minLength) {
      return { isValid: false, error: `Minimum length is ${rules.minLength} characters` }
    }

    // Max length validation
    if (rules.maxLength && value.toString().length > rules.maxLength) {
      return { isValid: false, error: `Maximum length is ${rules.maxLength} characters` }
    }

    // Pattern validation
    if (rules.pattern && !rules.pattern.test(value.toString())) {
      return { isValid: false, error: 'Invalid format' }
    }

    // URL validation
    if (rules.url) {
      try {
        new URL(value.toString())
      } catch {
        return { isValid: false, error: 'Invalid URL format' }
      }
    }

    // Custom validation
    if (rules.custom) {
      const customError = rules.custom(value)
      if (customError) {
        return { isValid: false, error: customError }
      }
    }

    return { isValid: true, error: null }
  }

  function setFieldValue(name: string, value: any) {
    if (fields.value[name]) {
      fields.value[name].value = value
      fields.value[name].touched = true
    }
  }

  function setFieldError(name: string, error: string | null) {
    if (fields.value[name]) {
      fields.value[name].error = error
      fields.value[name].isValid = !error
    }
  }

  function setFieldValidating(name: string, validating: boolean) {
    if (fields.value[name]) {
      fields.value[name].validating = validating
    }
  }

  function validateAllFields(rules: Record<string, ValidationRule>): boolean {
    let isValid = true

    Object.keys(rules).forEach(name => {
      const result = validateField(name, rules[name])
      setFieldError(name, result.error)
      if (!result.isValid) {
        isValid = false
      }
    })

    return isValid
  }

  function resetField(name: string) {
    if (fields.value[name]) {
      fields.value[name].error = null
      fields.value[name].touched = false
      fields.value[name].validating = false
      fields.value[name].isValid = true
    }
  }

  function resetAllFields() {
    Object.keys(fields.value).forEach(name => resetField(name))
  }

  const isFormValid = computed(() => {
    return Object.values(fields.value).every(field => field.isValid && !field.validating)
  })

  const hasErrors = computed(() => {
    return Object.values(fields.value).some(field => field.error !== null)
  })

  return {
    fields,
    registerField,
    validateField,
    setFieldValue,
    setFieldError,
    setFieldValidating,
    validateAllFields,
    resetField,
    resetAllFields,
    isFormValid,
    hasErrors,
  }
}

// URL reachability check with timeout
export async function checkUrlReachability(url: string, timeout = 5000): Promise<boolean> {
  try {
    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), timeout)

    const response = await fetch(url, {
      method: 'HEAD',
      mode: 'no-cors', // Allow cross-origin requests
      signal: controller.signal,
    })

    clearTimeout(timeoutId)

    // With no-cors mode, we can't check status, so if we didn't error, assume it's reachable
    return true
  } catch (error) {
    // If it's an abort error, the timeout was reached
    if (error instanceof Error && error.name === 'AbortError') {
      return false
    }
    // For other errors (network issues), consider unreachable
    return false
  }
}

// Auto-generate source name from URL
export function generateSourceNameFromUrl(url: string): string {
  try {
    const parsedUrl = new URL(url)
    const hostname = parsedUrl.hostname

    // Remove www. prefix if present
    const cleanHostname = hostname.replace(/^www\./, '')

    // Replace dots and hyphens with underscores, remove TLD
    const baseName = cleanHostname.split('.')[0]
    const sourceName = baseName.replace(/[.-]/g, '_')

    return sourceName
  } catch {
    // If URL parsing fails, return empty string
    return ''
  }
}

// Detect category from URL or meta tags (basic heuristics)
export function detectCategory(url: string): string {
  const urlLower = url.toLowerCase()

  if (urlLower.includes('news') || urlLower.includes('press')) {
    return 'News'
  }
  if (urlLower.includes('blog')) {
    return 'Blog'
  }
  if (urlLower.includes('.gov') || urlLower.includes('government')) {
    return 'Government'
  }
  if (urlLower.includes('.org')) {
    return 'Organization'
  }

  return 'Other'
}

// Parse cron expression and show next run time
export function parseNextRunTime(cronExpression: string): string | null {
  // This is a simplified parser - in production, use a library like cron-parser
  try {
    const parts = cronExpression.split(' ')
    if (parts.length !== 5) {
      return null
    }

    const [minute, hour, dayOfMonth, month, dayOfWeek] = parts

    // Handle common patterns
    if (minute === '0' && hour === '0') {
      return 'Daily at midnight'
    }
    if (minute === '0' && hour.startsWith('*/')) {
      const interval = parseInt(hour.substring(2))
      return `Every ${interval} hours`
    }
    if (minute.startsWith('*/')) {
      const interval = parseInt(minute.substring(2))
      return `Every ${interval} minutes`
    }
    if (minute === '0' && !hour.includes('*')) {
      return `Daily at ${hour}:00`
    }

    return 'Custom schedule'
  } catch {
    return null
  }
}
