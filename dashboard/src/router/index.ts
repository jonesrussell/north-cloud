import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

// Views
import PipelineMonitorView from '../views/PipelineMonitorView.vue'
import LoginView from '../views/LoginView.vue'
import NotFoundView from '../views/NotFoundView.vue'

// Content Intake views (formerly Crawler)
import JobsView from '../views/intake/JobsView.vue'
import JobDetailView from '../views/intake/JobDetailView.vue'
import QueuedLinksView from '../views/intake/QueuedLinksView.vue'
import RulesView from '../views/intake/RulesView.vue'

// Source Scheduling views (formerly Sources + Classifier Sources)
import SourcesView from '../views/scheduling/SourcesView.vue'
import SourceFormView from '../views/scheduling/SourceFormView.vue'
import CitiesView from '../views/scheduling/CitiesView.vue'
import ReputationView from '../views/scheduling/ReputationView.vue'

// Content Intelligence views (Classifier + Indexes)
import ClassifierStatsView from '../views/intelligence/ClassifierStatsView.vue'
import IndexesView from '../views/intelligence/IndexesView.vue'
import IndexDetailView from '../views/intelligence/IndexDetailView.vue'
import DocumentDetailView from '../views/intelligence/DocumentDetailView.vue'

// Distribution Engine views (formerly Publisher)
import RoutesView from '../views/distribution/RoutesView.vue'
import ChannelsView from '../views/distribution/ChannelsView.vue'
import ArticlesView from '../views/distribution/ArticlesView.vue'

// External Feeds views (new)
import RedisStreamsView from '../views/feeds/RedisStreamsView.vue'
import DeliveryLogsView from '../views/feeds/DeliveryLogsView.vue'

// System Overview views (new)
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

  // Pipeline Monitor (Dashboard) - root route
  {
    path: '/',
    name: 'pipeline-monitor',
    component: PipelineMonitorView,
    meta: { title: 'Pipeline Monitor', requiresAuth: true },
  },

  // ==========================================
  // Content Intake (formerly Crawler)
  // ==========================================
  {
    path: '/intake/jobs',
    name: 'intake-jobs',
    component: JobsView,
    meta: { title: 'Crawl Jobs', section: 'intake', requiresAuth: true },
  },
  {
    path: '/intake/jobs/new',
    name: 'intake-jobs-new',
    component: JobDetailView,
    props: { isNew: true },
    meta: { title: 'New Crawl Job', section: 'intake', requiresAuth: true },
  },
  {
    path: '/intake/jobs/:id',
    name: 'intake-job-detail',
    component: JobDetailView,
    props: true,
    meta: { title: 'Job Details', section: 'intake', requiresAuth: true },
  },
  {
    path: '/intake/queued-links',
    name: 'intake-queued-links',
    component: QueuedLinksView,
    meta: { title: 'Queued Links', section: 'intake', requiresAuth: true },
  },
  {
    path: '/intake/rules',
    name: 'intake-rules',
    component: RulesView,
    meta: { title: 'Classification Rules', section: 'intake', requiresAuth: true },
  },

  // ==========================================
  // Source Scheduling
  // ==========================================
  {
    path: '/scheduling/sources',
    name: 'scheduling-sources',
    component: SourcesView,
    meta: { title: 'Sources', section: 'scheduling', requiresAuth: true },
  },
  {
    path: '/scheduling/sources/new',
    name: 'scheduling-sources-new',
    component: SourceFormView,
    meta: { title: 'New Source', section: 'scheduling', requiresAuth: true },
  },
  {
    path: '/scheduling/sources/:id/edit',
    name: 'scheduling-sources-edit',
    component: SourceFormView,
    props: true,
    meta: { title: 'Edit Source', section: 'scheduling', requiresAuth: true },
  },
  {
    path: '/scheduling/cities',
    name: 'scheduling-cities',
    component: CitiesView,
    meta: { title: 'Cities', section: 'scheduling', requiresAuth: true },
  },
  {
    path: '/scheduling/reputation',
    name: 'scheduling-reputation',
    component: ReputationView,
    meta: { title: 'Source Reputation', section: 'scheduling', requiresAuth: true },
  },

  // ==========================================
  // Content Intelligence
  // ==========================================
  {
    path: '/intelligence/stats',
    name: 'intelligence-stats',
    component: ClassifierStatsView,
    meta: { title: 'Classifier Stats', section: 'intelligence', requiresAuth: true },
  },
  {
    path: '/intelligence/indexes',
    name: 'intelligence-indexes',
    component: IndexesView,
    meta: { title: 'Elasticsearch Indexes', section: 'intelligence', requiresAuth: true },
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

  // ==========================================
  // Distribution Engine (formerly Publisher)
  // ==========================================
  {
    path: '/distribution/routes',
    name: 'distribution-routes',
    component: RoutesView,
    meta: { title: 'Routes', section: 'distribution', requiresAuth: true },
  },
  {
    path: '/distribution/routes/new',
    name: 'distribution-routes-new',
    component: RoutesView,
    props: { showCreateModal: true },
    meta: { title: 'New Route', section: 'distribution', requiresAuth: true },
  },
  {
    path: '/distribution/channels',
    name: 'distribution-channels',
    component: ChannelsView,
    meta: { title: 'Channels', section: 'distribution', requiresAuth: true },
  },
  {
    path: '/distribution/articles',
    name: 'distribution-articles',
    component: ArticlesView,
    meta: { title: 'Recent Articles', section: 'distribution', requiresAuth: true },
  },

  // ==========================================
  // External Feeds
  // ==========================================
  {
    path: '/feeds/streams',
    name: 'feeds-streams',
    component: RedisStreamsView,
    meta: { title: 'Redis Streams', section: 'feeds', requiresAuth: true },
  },
  {
    path: '/feeds/logs',
    name: 'feeds-logs',
    component: DeliveryLogsView,
    meta: { title: 'Delivery Logs', section: 'feeds', requiresAuth: true },
  },

  // ==========================================
  // System Overview
  // ==========================================
  {
    path: '/system/health',
    name: 'system-health',
    component: HealthView,
    meta: { title: 'System Health', section: 'system', requiresAuth: true },
  },
  {
    path: '/system/auth',
    name: 'system-auth',
    component: AuthView,
    meta: { title: 'Authentication', section: 'system', requiresAuth: true },
  },
  {
    path: '/system/cache',
    name: 'system-cache',
    component: CacheView,
    meta: { title: 'Cache Status', section: 'system', requiresAuth: true },
  },

  // ==========================================
  // Legacy Redirects (backward compatibility)
  // ==========================================
  { path: '/crawler/jobs', redirect: '/intake/jobs' },
  { path: '/crawler/jobs/:id', redirect: (to) => `/intake/jobs/${to.params.id}` },
  { path: '/crawler/queued-links', redirect: '/intake/queued-links' },
  { path: '/crawler/stats', redirect: '/intelligence/stats' },
  { path: '/sources', redirect: '/scheduling/sources' },
  { path: '/sources/new', redirect: '/scheduling/sources/new' },
  { path: '/sources/:id/edit', redirect: (to) => `/scheduling/sources/${to.params.id}/edit` },
  { path: '/sources/cities', redirect: '/scheduling/cities' },
  { path: '/indexes', redirect: '/intelligence/indexes' },
  { path: '/indexes/:index_name', redirect: (to) => `/intelligence/indexes/${to.params.index_name}` },
  { path: '/classifier/rules', redirect: '/intake/rules' },
  { path: '/classifier/sources', redirect: '/scheduling/reputation' },
  { path: '/classifier/stats', redirect: '/intelligence/stats' },
  { path: '/publisher', redirect: '/distribution/routes' },
  { path: '/publisher/sources', redirect: '/distribution/routes' },
  { path: '/publisher/channels', redirect: '/distribution/channels' },
  { path: '/publisher/routes', redirect: '/distribution/routes' },
  { path: '/publisher/articles', redirect: '/distribution/articles' },
  { path: '/publisher/stats', redirect: '/intelligence/stats' },
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
