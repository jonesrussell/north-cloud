import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import './style.css'

import DashboardView from './views/DashboardView.vue'
import CrawlJobsView from './views/CrawlJobsView.vue'
import StatsView from './views/StatsView.vue'

const routes = [
  { path: '/', redirect: '/dashboard' },
  { path: '/dashboard', name: 'dashboard', component: DashboardView },
  { path: '/jobs', name: 'jobs', component: CrawlJobsView },
  { path: '/stats', name: 'stats', component: StatsView },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

createApp(App).use(router).mount('#app')
