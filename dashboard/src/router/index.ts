import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

// Views
import DashboardView from '../views/DashboardView.vue'

// Crawler views
import CrawlerJobsView from '../views/crawler/JobsView.vue'
import CrawlerStatsView from '../views/crawler/StatsView.vue'

// Publisher views
import PublisherStatsView from '../views/publisher/StatsView.vue'
import PublisherRecentArticlesView from '../views/publisher/RecentArticlesView.vue'
import PublisherDashboardView from '../views/publisher/PublisherDashboardView.vue'
import PublisherSourcesView from '../views/publisher/SourcesView.vue'
import PublisherChannelsView from '../views/publisher/ChannelsView.vue'
import PublisherRoutesView from '../views/publisher/RoutesView.vue'

// Sources views
import SourcesListView from '../views/sources/ListView.vue'
import SourcesFormView from '../views/sources/FormView.vue'
import CitiesView from '../views/sources/CitiesView.vue'

// Classifier views
import ClassifierStatsView from '../views/classifier/StatsView.vue'
import ClassifierRulesView from '../views/classifier/RulesView.vue'
import ClassifierSourceReputationView from '../views/classifier/SourceReputationView.vue'

// Analytics view (consolidated stats)
import AnalyticsView from '../views/AnalyticsView.vue'

// 404 view
import NotFoundView from '../views/NotFoundView.vue'

// Login view
import LoginView from '../views/LoginView.vue'

// Extend RouteMeta to include our custom properties
declare module 'vue-router' {
  interface RouteMeta {
    title?: string
    section?: string
    requiresAuth?: boolean
    breadcrumbs?: Array<{
      label: string
      path: string
      icon?: any
    }>
  }
}

const routes: RouteRecordRaw[] = [
  // Login route (public)
  {
    path: '/login',
    name: 'login',
    component: LoginView,
    meta: { title: 'Login', requiresAuth: false },
  },

  // Dashboard (Overview) - this is the root route for the app
  {
    path: '/',
    name: 'dashboard',
    component: DashboardView,
    meta: { title: 'Dashboard', requiresAuth: true },
  },

  // Analytics (Consolidated Statistics)
  {
    path: '/analytics',
    name: 'analytics',
    component: AnalyticsView,
    meta: { title: 'System Analytics', section: 'analytics', requiresAuth: true },
  },

  // Crawler routes
  {
    path: '/crawler/jobs',
    name: 'crawler-jobs',
    component: CrawlerJobsView,
    meta: { title: 'Crawl Jobs', section: 'crawler', requiresAuth: true },
  },
  // Redirect old crawler stats to analytics
  {
    path: '/crawler/stats',
    redirect: '/analytics?tab=crawler',
  },

  // Publisher routes
  {
    path: '/publisher',
    name: 'publisher-dashboard',
    component: PublisherDashboardView,
    meta: { title: 'Publisher Dashboard', section: 'publisher', requiresAuth: true },
  },
  {
    path: '/publisher/sources',
    name: 'publisher-sources',
    component: PublisherSourcesView,
    meta: { title: 'Publisher Sources', section: 'publisher', requiresAuth: true },
  },
  {
    path: '/publisher/channels',
    name: 'publisher-channels',
    component: PublisherChannelsView,
    meta: { title: 'Publisher Channels', section: 'publisher', requiresAuth: true },
  },
  {
    path: '/publisher/routes',
    name: 'publisher-routes',
    component: PublisherRoutesView,
    meta: { title: 'Publisher Routes', section: 'publisher', requiresAuth: true },
  },
  // Redirect old publisher stats to analytics
  {
    path: '/publisher/stats',
    redirect: '/analytics?tab=publisher',
  },
  {
    path: '/publisher/articles',
    name: 'publisher-articles',
    component: PublisherRecentArticlesView,
    meta: { title: 'Recent Articles', section: 'publisher', requiresAuth: true },
  },

  // Sources routes
  {
    path: '/sources',
    name: 'sources',
    component: SourcesListView,
    meta: { title: 'Sources', section: 'sources', requiresAuth: true },
  },
  {
    path: '/sources/new',
    name: 'source-new',
    component: SourcesFormView,
    meta: { title: 'New Source', section: 'sources', requiresAuth: true },
  },
  {
    path: '/sources/:id/edit',
    name: 'source-edit',
    component: SourcesFormView,
    props: true,
    meta: { title: 'Edit Source', section: 'sources', requiresAuth: true },
  },
  {
    path: '/sources/cities',
    name: 'cities',
    component: CitiesView,
    meta: { title: 'Cities', section: 'sources', requiresAuth: true },
  },

  // Classifier routes
  // Redirect old classifier stats to analytics
  {
    path: '/classifier/stats',
    redirect: '/analytics?tab=classifier',
  },
  {
    path: '/classifier/rules',
    name: 'classifier-rules',
    component: ClassifierRulesView,
    meta: { title: 'Classification Rules', section: 'classifier', requiresAuth: true },
  },
  {
    path: '/classifier/sources',
    name: 'classifier-sources',
    component: ClassifierSourceReputationView,
    meta: { title: 'Source Reputation', section: 'classifier', requiresAuth: true },
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
  history: createWebHistory('/dashboard/'),
  routes,
})

// Route guard for authentication
router.beforeEach((to, from, next) => {
  // Check if route requires authentication
  const requiresAuth = to.meta.requiresAuth !== false // Default to true unless explicitly false
  
  if (requiresAuth) {
    // Check if user is authenticated
    const token = localStorage.getItem('dashboard_token')
    if (!token) {
      // Redirect to login
      next({ name: 'login', query: { redirect: to.fullPath } })
      return
    }
  } else if (to.name === 'login') {
    // If already authenticated, redirect to dashboard
    const token = localStorage.getItem('dashboard_token')
    if (token) {
      next({ name: 'dashboard' })
      return
    }
  }
  
  next()
})

// Update document title and track recent pages on navigation
router.afterEach((to) => {
  // Update document title
  document.title = to.meta.title
    ? `${to.meta.title} - North Cloud`
    : 'North Cloud Dashboard'

  // Track recent page (dynamically import to avoid circular dependency)
  if (to.path !== '/login' && to.meta.title) {
    import('@/composables/useRecentPages').then(({ useRecentPages }) => {
      const { addRecentPage } = useRecentPages()
      addRecentPage({
        path: to.path,
        title: to.meta.title as string,
      })
    })
  }
})

export default router

