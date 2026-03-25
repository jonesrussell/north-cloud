export const endpoints = {
  auth: { login: '/api/auth/login' },
  sources: {
    list: '/api/sources',
    detail: (id: string) => `/api/sources/${id}`,
    create: '/api/sources',
    update: (id: string) => `/api/sources/${id}`,
    delete: (id: string) => `/api/sources/${id}`,
    enable: (id: string) => `/api/sources/${id}/enable`,
    disable: (id: string) => `/api/sources/${id}/disable`,
    testCrawl: '/api/sources/test-crawl',
    fetchMetadata: '/api/sources/fetch-metadata',
  },
  crawler: {
    jobs: '/api/crawler/jobs',
    job: (id: string) => `/api/crawler/jobs/${id}`,
  },
  publisher: {
    channels: '/api/publisher/channels',
    channel: (id: string) => `/api/publisher/channels/${id}`,
  },
  classifier: {
    rules: '/api/classifier/rules',
    rule: (id: string) => `/api/classifier/rules/${id}`,
  },
  indexManager: {
    indexes: '/api/index-manager/indexes',
    index: (name: string) => `/api/index-manager/indexes/${name}`,
  },
  search: {
    content: '/api/search/feeds/latest',
    feed: (slug: string) => `/api/search/feeds/${slug}`,
  },
} as const
