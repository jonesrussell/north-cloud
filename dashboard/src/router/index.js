import { createRouter, createWebHistory } from 'vue-router'

// Views
import DashboardView from '../views/DashboardView.vue'

// Crawler views
import CrawlerJobsView from '../views/crawler/JobsView.vue'
import CrawlerStatsView from '../views/crawler/StatsView.vue'

// Publisher views
import PublisherStatsView from '../views/publisher/StatsView.vue'
import PublisherRecentArticlesView from '../views/publisher/RecentArticlesView.vue'

// Sources views
import SourcesListView from '../views/sources/ListView.vue'
import SourcesFormView from '../views/sources/FormView.vue'
import CitiesView from '../views/sources/CitiesView.vue'

// Classifier views
import ClassifierStatsView from '../views/classifier/StatsView.vue'
import ClassifierRulesView from '../views/classifier/RulesView.vue'
import ClassifierSourceReputationView from '../views/classifier/SourceReputationView.vue'

// 404 view
import NotFoundView from '../views/NotFoundView.vue'

// Login view
import LoginView from '../views/LoginView.vue'

const routes = [
  // Root redirect
  {
    path: '/',
    redirect: '/dashboard',
  },

  // Login route (public)
  {
    path: '/login',
    name: 'login',
    component: LoginView,
    meta: { title: 'Login', public: true },
  },

  // Dashboard (Overview)
  {
    path: '/dashboard',
    name: 'dashboard',
    component: DashboardView,
    meta: { title: 'Dashboard' },
  },

  // Crawler routes
  {
    path: '/crawler/jobs',
    name: 'crawler-jobs',
    component: CrawlerJobsView,
    meta: { title: 'Crawl Jobs', section: 'crawler' },
  },
  {
    path: '/crawler/stats',
    name: 'crawler-stats',
    component: CrawlerStatsView,
    meta: { title: 'Crawler Statistics', section: 'crawler' },
  },

  // Publisher routes
  {
    path: '/publisher/stats',
    name: 'publisher-stats',
    component: PublisherStatsView,
    meta: { title: 'Publisher Statistics', section: 'publisher' },
  },
  {
    path: '/publisher/articles',
    name: 'publisher-articles',
    component: PublisherRecentArticlesView,
    meta: { title: 'Recent Articles', section: 'publisher' },
  },

  // Sources routes
  {
    path: '/sources',
    name: 'sources',
    component: SourcesListView,
    meta: { title: 'Sources', section: 'sources' },
  },
  {
    path: '/sources/new',
    name: 'source-new',
    component: SourcesFormView,
    meta: { title: 'New Source', section: 'sources' },
  },
  {
    path: '/sources/:id/edit',
    name: 'source-edit',
    component: SourcesFormView,
    props: true,
    meta: { title: 'Edit Source', section: 'sources' },
  },
  {
    path: '/sources/cities',
    name: 'cities',
    component: CitiesView,
    meta: { title: 'Cities', section: 'sources' },
  },

  // Classifier routes
  {
    path: '/classifier/stats',
    name: 'classifier-stats',
    component: ClassifierStatsView,
    meta: { title: 'Classifier Statistics', section: 'classifier' },
  },
  {
    path: '/classifier/rules',
    name: 'classifier-rules',
    component: ClassifierRulesView,
    meta: { title: 'Classification Rules', section: 'classifier' },
  },
  {
    path: '/classifier/sources',
    name: 'classifier-sources',
    component: ClassifierSourceReputationView,
    meta: { title: 'Source Reputation', section: 'classifier' },
  },

  // 404 catch-all route - must be last
  {
    path: '/:pathMatch(.*)*',
    name: 'not-found',
    component: NotFoundView,
    meta: { title: 'Page Not Found' },
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

// Auth guard - protect all routes except public ones
router.beforeEach(async (to, from, next) => {
  const { useAuth } = await import('../composables/useAuth')
  const auth = useAuth()
  const { isAuthenticated, validate, token, refreshToken, user } = auth

  // Check if route is public
  if (to.meta.public) {
    // If already authenticated and trying to access login, redirect to dashboard
    if (to.path === '/login' && isAuthenticated.value) {
      next('/dashboard')
      return
    }
    next()
    return
  }

  // Protected route - check authentication
  // First check if we have a token at all
  const hasToken = !!token.value || !!localStorage.getItem('auth_token')
  
  if (!hasToken || !isAuthenticated.value) {
    // No token, redirect to login immediately
    console.log('[Router Guard] No token found, redirecting to login')
    next({
      path: '/login',
      query: { redirect: to.fullPath },
    })
    return
  }

  // We have a token, validate it
  try {
    // Pass skipLogout=true to prevent validate() from calling logout() which redirects
    // The router guard will handle the redirect
    const isValid = await validate(true)
    if (!isValid) {
      // Token invalid or expired, clear state and redirect to login
      token.value = null
      refreshToken.value = null
      user.value = null
      localStorage.removeItem('auth_token')
      localStorage.removeItem('auth_refresh_token')
      localStorage.removeItem('auth_user')
      
      next({
        path: '/login',
        query: { redirect: to.fullPath },
      })
      return
    }
  } catch (error) {
    // Validation failed (network error, etc.), clear state and redirect to login
    console.error('Auth validation error:', error)
    token.value = null
    refreshToken.value = null
    user.value = null
    localStorage.removeItem('auth_token')
    localStorage.removeItem('auth_refresh_token')
    localStorage.removeItem('auth_user')
    
    next({
      path: '/login',
      query: { redirect: to.fullPath },
    })
    return
  }

  // Authentication valid, allow navigation
  next()
})

// Update document title on navigation
router.afterEach((to) => {
  document.title = to.meta.title
    ? `${to.meta.title} - North Cloud`
    : 'North Cloud Dashboard'
})

export default router
