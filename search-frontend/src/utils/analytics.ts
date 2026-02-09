/**
 * Lightweight analytics hook. Fire events for search submit, filter changes, suggestion selection.
 * No backend required; replace with your analytics provider (e.g. GA, Plausible) when ready.
 */
export type AnalyticsEventName =
  | 'search_submit'
  | 'search_suggestion_select'
  | 'filter_change'
  | 'result_click'

export interface AnalyticsPayload {
  query?: string
  suggestion?: string
  filter_type?: string
  filter_value?: string | string[]
  result_id?: string
  [key: string]: unknown
}

export function trackEvent(eventName: AnalyticsEventName, payload?: AnalyticsPayload): void {
  if (import.meta.env.DEV) {
    console.debug('[Analytics]', eventName, payload ?? {})
  }
  // Optional: dispatch custom event for parent/analytics scripts
  try {
    window.dispatchEvent(
      new CustomEvent('north-cloud-analytics', {
        detail: { eventName, payload: payload ?? {} },
      })
    )
  } catch {
    // ignore
  }
}

export default trackEvent
