import type { NavigationGuardWithThis } from 'vue-router'
import { useAuth } from './useAuth'

export const authGuard: NavigationGuardWithThis<undefined> = (to) => {
  const { isAuthenticated } = useAuth()
  if (to.meta.requiresAuth !== false && !isAuthenticated.value) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  return true
}
