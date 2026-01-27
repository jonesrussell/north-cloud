import { VueQueryPlugin, QueryClient, type VueQueryPluginOptions } from '@tanstack/vue-query'
import type { App } from 'vue'

/**
 * Global QueryClient configuration for TanStack Query
 *
 * Key settings:
 * - staleTime: How long data is considered fresh (5 minutes)
 * - gcTime: How long inactive data stays in cache (10 minutes)
 * - retry: Number of retries for failed requests (1)
 * - refetchOnWindowFocus: Disabled by default (opt-in per query)
 */
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // Data is fresh for 5 minutes
      staleTime: 5 * 60 * 1000,
      // Keep unused data in cache for 10 minutes
      gcTime: 10 * 60 * 1000,
      // Retry failed requests once
      retry: 1,
      // Don't refetch on window focus by default
      refetchOnWindowFocus: false,
      // Refetch when reconnecting
      refetchOnReconnect: true,
      // Don't throw errors, handle them in components
      throwOnError: false,
    },
    mutations: {
      // Don't retry mutations (explicit user actions)
      retry: 0,
    },
  },
})

export const vueQueryOptions: VueQueryPluginOptions = {
  queryClient,
}

/**
 * Install Vue Query plugin
 */
export function installVueQuery(app: App): void {
  app.use(VueQueryPlugin, vueQueryOptions)
}

export default queryClient
