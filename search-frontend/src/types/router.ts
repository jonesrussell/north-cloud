import 'vue-router'

/**
 * Route meta interface for type-safe route metadata
 */
declare module 'vue-router' {
  interface RouteMeta {
    title?: string
    [key: string]: unknown
  }
}

