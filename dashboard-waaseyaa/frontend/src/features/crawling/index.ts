export const crawlingRoutes = [
  {
    path: 'crawl-jobs',
    name: 'crawl-jobs',
    component: () => import('./views/JobList.vue'),
  },
  {
    path: 'crawl-jobs/:id',
    name: 'crawl-job-detail',
    component: () => import('./views/JobDetail.vue'),
  },
]
