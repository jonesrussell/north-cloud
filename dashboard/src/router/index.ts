import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

// Views
import PipelineMonitorView from '../views/PipelineMonitorView.vue'
import LoginView from '../views/LoginView.vue'
import NotFoundView from '../views/NotFoundView.vue'

// Operations views
import ArticlesView from '../views/distribution/ArticlesView.vue'

// Content Intake views
import JobsView from '../views/intake/JobsView.vue'
import JobDetailView from '../views/intake/JobDetailView.vue'
import DiscoveredLinksView from '../views/intake/DiscoveredLinksView.vue'
import RulesView from '../views/intake/RulesView.vue'

// Sources views (consolidated from scheduling)
import SourcesView from '../views/scheduling/SourcesView.vue'
import SourceFormView from '../views/scheduling/SourceFormView.vue'
import CitiesView from '../views/scheduling/CitiesView.vue'
import ReputationView from '../views/scheduling/ReputationView.vue'

// Intelligence views
import ClassifierStatsView from '../views/intelligence/ClassifierStatsView.vue'
import IndexesView from '../views/intelligence/IndexesView.vue'
import IndexDetailView from '../views/intelligence/IndexDetailView.vue'
import DocumentDetailView from '../views/intelligence/DocumentDetailView.vue'

// Distribution views
import ChannelsView from '../views/distribution/ChannelsView.vue'
import DeliveryLogsView from '../views/feeds/DeliveryLogsView.vue'

// System views
import HealthView from '../views/system/HealthView.vue'
import AuthView from '../views/system/AuthView.vue'
import CacheView from '../views/system/CacheView.vue'

// Extend RouteMeta to include our custom properties
declare module 'vue-router' {
  interface RouteMeta {
    title?: string
    section?: string
    requiresAuth?: boolean
    breadcrumbs?: Array<{
      label: string
      path: string
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

  // ==========================================
  // Operations - daily cockpit
  // ==========================================
  {
    path: '/',
    name: 'pipeline-monitor',
    component: PipelineMonitorView,
    meta: { title: 'Pipeline Monitor', section: 'operations', requiresAuth: true },
  },
  {
    path: '/operations/articles',
    name: 'operations-articles',
    component: ArticlesView,
    meta: { title: 'Recent Articles', section: 'operations', requiresAuth: true },
  },
  {
    path: '/operations/review',
    name: 'operations-review',
    // Lazy load - will be created in Task 13
    component: () => import('../views/operations/ReviewQueueView.vue'),
    meta: { title: 'Review Queue', section: 'operations', requiresAuth: true },
  },

  // ==========================================
  // Intelligence - overview and drill-downs
  // ==========================================
  {
    path: '/intelligence',
    name: 'intelligence-overview',
    component: () => import('../views/intelligence/IntelligenceOverviewView.vue'),
    meta: { title: 'Intelligence', section: 'intelligence', requiresAuth: true },
  },
  {
    path: '/intelligence/crime',
    name: 'intelligence-crime',
    // Lazy load - will be created in Task 14
    component: () => import('../views/intelligence/CrimeBreakdownView.vue'),
    meta: { title: 'Crime Breakdown', section: 'intelligence', requiresAuth: true },
  },
  {
    path: '/intelligence/mining',
    name: 'intelligence-mining',
    component: () => import('../views/intelligence/MiningBreakdownView.vue'),
    meta: { title: 'Mining Breakdown', section: 'intelligence', requiresAuth: true },
  },
  {
    path: '/intelligence/location',
    name: 'intelligence-location',
    // Lazy load - will be created in Task 15
    component: () => import('../views/intelligence/LocationBreakdownView.vue'),
    meta: { title: 'Location Breakdown', section: 'intelligence', requiresAuth: true },
  },
  {
    path: '/intelligence/indexes',
    name: 'intelligence-indexes',
    component: IndexesView,
    meta: { title: 'Index Explorer', section: 'intelligence', requiresAuth: true },
  },
  {
    path: '/intelligence/indexes/:index_name',
    name: 'intelligence-index-detail',
    component: IndexDetailView,
    props: true,
    meta: { title: 'Index Details', section: 'intelligence', requiresAuth: true },
  },
  {
    path: '/intelligence/indexes/:index_name/documents/:document_id',
    name: 'intelligence-document-detail',
    component: DocumentDetailView,
    props: true,
    meta: { title: 'Document Details', section: 'intelligence', requiresAuth: true },
  },
  // Legacy intelligence routes
  {
    path: '/intelligence/stats',
    name: 'intelligence-stats',
    component: ClassifierStatsView,
    meta: { title: 'Classifier Stats', section: 'intelligence', requiresAuth: true },
  },

  // ==========================================
  // Content Intake
  // ==========================================
  {
    path: '/intake/jobs',
    name: 'intake-jobs',
    component: JobsView,
    meta: { title: 'Crawler Jobs', section: 'intake', requiresAuth: true },
  },
  {
    path: '/intake/jobs/:id',
    name: 'intake-job-detail',
    component: JobDetailView,
    props: true,
    meta: { title: 'Job Details', section: 'intake', requiresAuth: true },
  },
  {
    path: '/intake/discovered-links',
    name: 'intake-discovered-links',
    component: DiscoveredLinksView,
    meta: { title: 'Discovered Links', section: 'intake', requiresAuth: true },
  },
  {
    path: '/intake/frontier',
    name: 'intake-frontier',
    component: () => import('../views/intake/FrontierView.vue'),
    meta: { title: 'URL Frontier', section: 'intake', requiresAuth: true },
  },
  {
    path: '/intake/rules',
    name: 'intake-rules',
    component: RulesView,
    meta: { title: 'Rules', section: 'intake', requiresAuth: true },
  },

  // ==========================================
  // Sources (consolidated from scheduling)
  // ==========================================
  {
    path: '/sources',
    name: 'sources',
    component: SourcesView,
    meta: { title: 'All Sources', section: 'sources', requiresAuth: true },
  },
  {
    path: '/sources/new',
    name: 'sources-new',
    component: SourceFormView,
    meta: { title: 'New Source', section: 'sources', requiresAuth: true },
  },
  {
    path: '/sources/:id/edit',
    name: 'sources-edit',
    component: SourceFormView,
    props: true,
    meta: { title: 'Edit Source', section: 'sources', requiresAuth: true },
  },
  {
    path: '/sources/cities',
    name: 'sources-cities',
    component: CitiesView,
    meta: { title: 'Cities', section: 'sources', requiresAuth: true },
  },
  {
    path: '/sources/reputation',
    name: 'sources-reputation',
    component: ReputationView,
    meta: { title: 'Reputation', section: 'sources', requiresAuth: true },
  },

  // ==========================================
  // Distribution
  // ==========================================
  {
    path: '/distribution/channels',
    name: 'distribution-channels',
    component: ChannelsView,
    meta: { title: 'Channels', section: 'distribution', requiresAuth: true },
  },
  {
    path: '/distribution/routes',
    name: 'distribution-routes',
    // Lazy load - will be created in Task 16
    component: () => import('../views/distribution/RoutesView.vue'),
    meta: { title: 'Routes', section: 'distribution', requiresAuth: true },
  },
  {
    path: '/distribution/logs',
    name: 'distribution-logs',
    component: DeliveryLogsView,
    meta: { title: 'Delivery Logs', section: 'distribution', requiresAuth: true },
  },

  // ==========================================
  // System
  // ==========================================
  {
    path: '/system/health',
    name: 'system-health',
    component: HealthView,
    meta: { title: 'Health', section: 'system', requiresAuth: true },
  },
  {
    path: '/system/auth',
    name: 'system-auth',
    component: AuthView,
    meta: { title: 'Auth', section: 'system', requiresAuth: true },
  },
  {
    path: '/system/cache',
    name: 'system-cache',
    component: CacheView,
    meta: { title: 'Cache', section: 'system', requiresAuth: true },
  },

  // ==========================================
  // Legacy Redirects (backward compatibility)
  // ==========================================
  // Old crawler routes
  { path: '/crawler/jobs', redirect: '/intake/jobs' },
  { path: '/crawler/jobs/:id', redirect: (to) => `/intake/jobs/${to.params.id}` },
  { path: '/crawler/discovered-links', redirect: '/intake/discovered-links' },
  { path: '/crawler/stats', redirect: '/intelligence/stats' },

  // Old scheduling routes -> sources
  { path: '/scheduling/sources', redirect: '/sources' },
  { path: '/scheduling/sources/new', redirect: '/sources/new' },
  { path: '/scheduling/sources/:id/edit', redirect: (to) => `/sources/${to.params.id}/edit` },
  { path: '/scheduling/cities', redirect: '/sources/cities' },
  { path: '/scheduling/reputation', redirect: '/sources/reputation' },

  // Old indexes routes
  { path: '/indexes', redirect: '/intelligence/indexes' },
  { path: '/indexes/:index_name', redirect: (to) => `/intelligence/indexes/${to.params.index_name}` },

  // Old classifier routes
  { path: '/classifier/rules', redirect: '/intake/rules' },
  { path: '/classifier/sources', redirect: '/sources/reputation' },
  { path: '/classifier/stats', redirect: '/intelligence/stats' },

  // Old publisher routes
  { path: '/publisher', redirect: '/distribution/channels' },
  { path: '/publisher/sources', redirect: '/distribution/channels' },
  { path: '/publisher/channels', redirect: '/distribution/channels' },
  { path: '/publisher/routes', redirect: '/distribution/routes' },
  { path: '/publisher/articles', redirect: '/operations/articles' },
  { path: '/publisher/stats', redirect: '/intelligence/stats' },

  // Old distribution routes
  { path: '/distribution/articles', redirect: '/operations/articles' },

  // Old feeds routes
  { path: '/feeds/streams', redirect: '/distribution/logs' },
  { path: '/feeds/logs', redirect: '/distribution/logs' },

  // Old analytics route
  { path: '/analytics', redirect: '/intelligence/stats' },

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
router.beforeEach((to, _from, next) => {
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
      next({ name: 'pipeline-monitor' })
      return
    }
  }

  next()
})

// Update document title on navigation
router.afterEach((to) => {
  document.title = to.meta.title ? `${to.meta.title} - North Cloud` : 'North Cloud Dashboard'
})

export default router
