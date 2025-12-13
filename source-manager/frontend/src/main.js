import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import './style.css'

import SourcesView from './views/SourcesView.vue'
import SourceFormView from './views/SourceFormView.vue'
import CitiesView from './views/CitiesView.vue'

const routes = [
  { path: '/', redirect: '/sources' },
  { path: '/sources', name: 'sources', component: SourcesView },
  { path: '/sources/new', name: 'source-new', component: SourceFormView },
  { path: '/sources/:id/edit', name: 'source-edit', component: SourceFormView, props: true },
  { path: '/cities', name: 'cities', component: CitiesView },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

createApp(App).use(router).mount('#app')

