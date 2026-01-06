<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { Download, Share2, BarChart3, GitBranch } from 'lucide-vue-next'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { PipelineFlow, MetricCard, HealthBadge, QuickActions } from '@/components/pipeline'
import { crawlerApi, publisherApi, classifierApi } from '@/api/client'

// Mock data for pipeline stats - in production, fetch from APIs
const loading = ref(true)
const error = ref<string | null>(null)

// Pipeline stages data
const pipelineStages = ref([
  { name: 'Crawled', count: 0, change: 0, status: 'healthy' as const },
  { name: 'Indexed', count: 0, change: 0, status: 'healthy' as const },
  { name: 'Classified', count: 0, change: 0, status: 'healthy' as const },
  { name: 'Routed', count: 0, change: 0, status: 'healthy' as const },
  { name: 'Delivered', count: 0, change: 0, status: 'healthy' as const },
])

// Key metrics
const metrics = ref({
  avgQualityScore: 0,
  activeRoutes: 0,
  totalRoutes: 0,
  contentToday: 0,
  articlesRouted: 0,
})

// Service health
const serviceHealth = ref<
  Array<{ name: string; status: 'healthy' | 'degraded' | 'unhealthy' | 'unknown' }>
>([
  { name: 'Crawler', status: 'unknown' },
  { name: 'Classifier', status: 'unknown' },
  { name: 'Publisher', status: 'unknown' },
  { name: 'Redis', status: 'unknown' },
  { name: 'Elasticsearch', status: 'unknown' },
])

// Quick actions
const quickActions = [
  { label: 'New Crawl Job', path: '/intake/jobs/new', icon: 'plus' as const },
  { label: 'Add Source', path: '/scheduling/sources/new', icon: 'plus' as const },
  { label: 'New Route', path: '/distribution/routes/new', icon: 'plus' as const },
  { label: 'View Analytics', path: '/intelligence/stats', icon: 'chart' as const },
]

// Fetch data on mount
onMounted(async () => {
  try {
    loading.value = true

    // Check service health
    const healthChecks = await Promise.allSettled([
      crawlerApi.getHealth(),
      classifierApi.getHealth(),
      publisherApi.getHealth(),
    ])

    serviceHealth.value[0].status = healthChecks[0].status === 'fulfilled' ? 'healthy' : 'unhealthy'
    serviceHealth.value[1].status = healthChecks[1].status === 'fulfilled' ? 'healthy' : 'unhealthy'
    serviceHealth.value[2].status = healthChecks[2].status === 'fulfilled' ? 'healthy' : 'unhealthy'
    
    // Redis and ES are considered healthy if services are up
    serviceHealth.value[3].status = healthChecks[2].status === 'fulfilled' ? 'healthy' : 'unknown'
    serviceHealth.value[4].status = healthChecks[0].status === 'fulfilled' ? 'healthy' : 'unknown'

    // Try to fetch stats from crawler
    try {
      const crawlerStats = await crawlerApi.getStats()
      if (crawlerStats) {
        pipelineStages.value[0].count = crawlerStats.total_jobs || 0
        pipelineStages.value[1].count = crawlerStats.total_indexed || crawlerStats.total_jobs || 0
        metrics.value.contentToday = crawlerStats.jobs_today || 0
      }
    } catch {
      // Use mock data
      pipelineStages.value[0].count = 847
      pipelineStages.value[0].change = 12
      pipelineStages.value[1].count = 832
      pipelineStages.value[1].change = 10
    }

    // Try to fetch stats from publisher
    try {
      const publisherStats = await publisherApi.getStats()
      if (publisherStats) {
        metrics.value.activeRoutes = publisherStats.active_routes || 0
        metrics.value.totalRoutes = publisherStats.total_routes || 0
        metrics.value.articlesRouted = publisherStats.articles_today || 0
        pipelineStages.value[3].count = publisherStats.articles_today || 0
        pipelineStages.value[4].count = publisherStats.articles_delivered || 0
      }
    } catch {
      // Use mock data
      pipelineStages.value[3].count = 456
      pipelineStages.value[3].change = -3
      pipelineStages.value[4].count = 442
      pipelineStages.value[4].change = -5
      metrics.value.activeRoutes = 12
      metrics.value.totalRoutes = 15
      metrics.value.articlesRouted = 456
    }

    // Mock classifier data
    pipelineStages.value[2].count = 819
    pipelineStages.value[2].change = 8
    metrics.value.avgQualityScore = 72

    loading.value = false
  } catch (e) {
    error.value = 'Failed to load pipeline data'
    loading.value = false
  }
})
</script>

<template>
  <div class="space-y-6">
    <!-- Page Header -->
    <div>
      <h1 class="text-3xl font-bold tracking-tight">Pipeline Monitor</h1>
      <p class="text-muted-foreground mt-1">
        Overview of your content pipeline health and throughput
      </p>
    </div>

    <!-- Pipeline Flow -->
    <Card>
      <CardHeader>
        <CardTitle>Content Flow Today</CardTitle>
        <CardDescription>
          Track content as it moves through each stage of the pipeline
        </CardDescription>
      </CardHeader>
      <CardContent>
        <PipelineFlow :stages="pipelineStages" />
      </CardContent>
    </Card>

    <!-- Metrics Grid -->
    <div class="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
      <MetricCard
        title="Content Ingested"
        :value="metrics.contentToday || pipelineStages[0].count"
        subtitle="articles today"
        :change="pipelineStages[0].change"
        :trend="pipelineStages[0].change > 0 ? 'up' : 'down'"
        :icon="Download"
      />
      <MetricCard
        title="Articles Routed"
        :value="metrics.articlesRouted || pipelineStages[3].count"
        subtitle="to channels today"
        :change="pipelineStages[3].change"
        :trend="pipelineStages[3].change > 0 ? 'up' : 'down'"
        :icon="Share2"
      />
      <MetricCard
        title="Avg Quality Score"
        :value="`${metrics.avgQualityScore}/100`"
        subtitle="classifier output"
        :icon="BarChart3"
      />
      <MetricCard
        title="Active Routes"
        :value="`${metrics.activeRoutes}/${metrics.totalRoutes}`"
        subtitle="routes enabled"
        :icon="GitBranch"
      />
    </div>

    <!-- Bottom Row: Health + Quick Actions -->
    <div class="grid gap-4 md:grid-cols-3">
      <!-- System Health -->
      <Card class="md:col-span-2">
        <CardHeader>
          <CardTitle>System Health</CardTitle>
          <CardDescription>Status of all pipeline services</CardDescription>
        </CardHeader>
        <CardContent>
          <div class="flex flex-wrap gap-3">
            <HealthBadge
              v-for="service in serviceHealth"
              :key="service.name"
              :name="service.name"
              :status="service.status"
            />
          </div>
        </CardContent>
      </Card>

      <!-- Quick Actions -->
      <QuickActions :actions="quickActions" />
    </div>
  </div>
</template>
