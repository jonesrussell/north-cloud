import { createRouter, createWebHistory } from 'vue-router'
import DashboardView from '../views/DashboardView.vue'
import SourcesView from '../views/SourcesView.vue'
import ChannelsView from '../views/ChannelsView.vue'
import RoutesView from '../views/RoutesView.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      name: 'dashboard',
      component: DashboardView
    },
    {
      path: '/sources',
      name: 'sources',
      component: SourcesView
    },
    {
      path: '/channels',
      name: 'channels',
      component: ChannelsView
    },
    {
      path: '/routes',
      name: 'routes',
      component: RoutesView
    }
  ]
})

export default router
