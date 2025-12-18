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

const routes = [
  // Root redirect
  {
    path: '/',
    redirect: '/dashboard',
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
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

// Update document title on navigation
router.afterEach((to) => {
  document.title = to.meta.title
    ? `${to.meta.title} - North Cloud`
    : 'North Cloud Dashboard'
})

export default router
