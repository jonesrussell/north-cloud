import { createRouter, createWebHistory } from 'vue-router'
import { authGuard } from '@/shared/auth/authGuard'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('@/features/auth/views/LoginView.vue'),
      meta: { requiresAuth: false },
    },
    {
      path: '/',
      component: () => import('@/layouts/AppShell.vue'),
      children: [
        {
          path: '',
          name: 'home',
          component: () => import('@/features/home/views/HomeView.vue'),
        },
        {
          path: 'sources',
          name: 'sources',
          component: () => import('@/features/sources/views/SourceList.vue'),
        },
        {
          path: 'sources/new',
          name: 'source-create',
          component: () => import('@/features/sources/views/SourceForm.vue'),
        },
        {
          path: 'sources/:id',
          name: 'source-detail',
          component: () => import('@/features/sources/views/SourceDetail.vue'),
        },
        {
          path: 'sources/:id/edit',
          name: 'source-edit',
          component: () => import('@/features/sources/views/SourceForm.vue'),
        },
        {
          path: 'crawl-jobs',
          name: 'crawl-jobs',
          component: () => import('@/features/crawling/views/JobList.vue'),
        },
        {
          path: 'crawl-jobs/:id',
          name: 'crawl-job-detail',
          component: () => import('@/features/crawling/views/JobDetail.vue'),
        },
      ],
    },
    {
      path: '/:pathMatch(.*)*',
      name: 'not-found',
      component: () => import('@/features/home/views/NotFoundView.vue'),
    },
  ],
})

router.beforeEach(authGuard)

export default router
