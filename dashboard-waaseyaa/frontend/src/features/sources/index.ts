import type { RouteRecordRaw } from 'vue-router'

export const sourceRoutes: RouteRecordRaw[] = [
  {
    path: 'sources',
    name: 'sources',
    component: () => import('./views/SourceList.vue'),
  },
  {
    path: 'sources/new',
    name: 'source-create',
    component: () => import('./views/SourceForm.vue'),
  },
  {
    path: 'sources/:id',
    name: 'source-detail',
    component: () => import('./views/SourceDetail.vue'),
  },
  {
    path: 'sources/:id/edit',
    name: 'source-edit',
    component: () => import('./views/SourceForm.vue'),
  },
]
