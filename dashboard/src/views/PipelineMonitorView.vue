<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { Download, Share2, BarChart3, GitBranch, Briefcase, ArrowRight } from 'lucide-vue-next'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { PipelineFlow, MetricCard, HealthBadge, QuickActions } from '@/components/pipeline'
import { JobStatsCard } from '@/components/domain/jobs'
import { useHealthStore, useMetricsStore } from '@/stores'
import { DEFAULT_QUICK_ACTIONS } from '@/types/metrics'

const router = useRouter()
const healthStore = useHealthStore()
const metricsStore = useMetricsStore()
// JobStatsCard now uses TanStack Query internally via useJobs()

// Computed values from stores
const pipelineStages = computed(() => metricsStore.pipelineStages)

const metrics = computed(() => ({
  contentToday: metricsStore.totalCrawledToday,
  articlesRouted: metricsStore.totalPublishedToday,
  avgQualityScore: metricsStore.avgQualityScore,
  activeRoutes: metricsStore.activeRoutes,
  totalRoutes: metricsStore.totalRoutes,
}))

// Service health from health store (first 5 services for compact display)
const serviceHealth = computed(() =>
  healthStore.services.slice(0, 5).map((s) => ({
    name: s.name,
    status: s.status === 'checking' ? 'unknown' : s.status,
  }))
)

const quickActions = DEFAULT_QUICK_ACTIONS

// Polling intervals
const HEALTH_INTERVAL = 30000 // 30 seconds
const METRICS_INTERVAL = 30000 // 30 seconds

onMounted(() => {
  // Start polling for health and metrics
  // Jobs data is automatically fetched by JobStatsCard via TanStack Query
  healthStore.startPolling(HEALTH_INTERVAL)
  metricsStore.startPolling(METRICS_INTERVAL)
})

onUnmounted(() => {
  // Stop polling when leaving the view
  healthStore.stopPolling()
  metricsStore.stopPolling()
})

function goToJobs() {
  router.push({ name: 'intake-jobs' })
}
</script>

<template>
  <div class="space-y-6">
    <!-- Page Header -->
    <div>
      <h1 class="text-3xl font-bold tracking-tight">
        Pipeline Monitor
      </h1>
      <p class="mt-1 text-muted-foreground">
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
        :value="metrics.contentToday"
        subtitle="articles today"
        :icon="Download"
      />
      <MetricCard
        title="Articles Routed"
        :value="metrics.articlesRouted"
        subtitle="to channels today"
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

    <!-- Jobs Summary -->
    <Card>
      <CardHeader class="flex flex-row items-center justify-between">
        <div>
          <CardTitle class="flex items-center gap-2">
            <Briefcase class="h-5 w-5" />
            Crawl Jobs
          </CardTitle>
          <CardDescription>Current status of all crawler jobs</CardDescription>
        </div>
        <Button
          variant="outline"
          size="sm"
          @click="goToJobs"
        >
          View All
          <ArrowRight class="ml-1 h-4 w-4" />
        </Button>
      </CardHeader>
      <CardContent>
        <JobStatsCard compact />
      </CardContent>
    </Card>

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
