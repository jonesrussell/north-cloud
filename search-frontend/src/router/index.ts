import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

// Views
import HomeView from '../views/HomeView.vue'
import ResultsView from '../views/ResultsView.vue'
import AdvancedSearchView from '../views/AdvancedSearchView.vue'
import NotFoundView from '../views/NotFoundView.vue'

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    name: 'home',
    component: HomeView,
    meta: { title: 'Search' },
  },
  {
    path: '/search',
    name: 'results',
    component: ResultsView,
    meta: { title: 'Search Results' },
  },
  {
    path: '/advanced',
    name: 'advanced',
    component: AdvancedSearchView,
    meta: { title: 'Advanced Search' },
  },
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
  scrollBehavior(to, from, savedPosition) {
    if (savedPosition) {
      return savedPosition
    } else {
      return { top: 0 }
    }
  },
})

// Update document title on navigation
router.afterEach((to) => {
  document.title = to.meta.title
    ? `${to.meta.title} - North Cloud`
    : 'North Cloud'
})

export default router

