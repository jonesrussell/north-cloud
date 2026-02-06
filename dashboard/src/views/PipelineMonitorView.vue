<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import {
  Download,
  Share2,
  BarChart3,
  GitBranch,
  Briefcase,
  ArrowRight,
  Brain,
  AlertTriangle,
  MapPin,
} from 'lucide-vue-next'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { PipelineFlow, MetricCard, HealthBadge, QuickActions } from '@/components/pipeline'
import { JobStatsCard } from '@/components/domain/jobs'
import { useHealthStore, useMetricsStore } from '@/stores'
import { DEFAULT_QUICK_ACTIONS } from '@/types/metrics'
import { indexManagerApi } from '@/api/client'
import type { OverviewAggregation } from '@/types/aggregation'

const router = useRouter()
const healthStore = useHealthStore()
const metricsStore = useMetricsStore()

// Intelligence overview from aggregations
const intelligenceOverview = ref<OverviewAggregation | null>(null)
const intelligenceLoading = ref(true)

const loadIntelligenceOverview = async () => {
  try {
    intelligenceLoading.value = true
    const response = await indexManagerApi.aggregations.getOverview()
    intelligenceOverview.value = response.data
  } catch (err) {
    console.error('Failed to load intelligence overview:', err)
  } finally {
    intelligenceLoading.value = false
  }
}

const crimePercentage = computed(() => {
  if (!intelligenceOverview.value) return 0
  const { total_crime_related, total_documents } = intelligenceOverview.value
  if (total_documents === 0) return 0
  return Math.round((total_crime_related / total_documents) * 100)
})

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
  healthStore.startPolling(HEALTH_INTERVAL)
  metricsStore.startPolling(METRICS_INTERVAL)
  loadIntelligenceOverview()
})

onUnmounted(() => {
  healthStore.stopPolling()
  metricsStore.stopPolling()
})

function goToJobs() {
  router.push({ name: 'intake-jobs' })
}
</script>

<template>
  <div class="space-y-6 animate-fade-up">
    <!-- Page Header -->
    <div>
      <h1 class="text-2xl font-semibold tracking-tight">
        Pipeline Monitor
      </h1>
      <p class="mt-0.5 text-sm text-muted-foreground">
        Content pipeline health and throughput
      </p>
    </div>

    <!-- Pipeline Flow -->
    <Card>
      <CardHeader class="pb-3">
        <CardTitle class="text-sm font-medium uppercase tracking-wider text-muted-foreground">
          Content Flow Today
        </CardTitle>
        <CardDescription class="text-xs">
          Track content as it moves through each stage. &quot;Today&quot; is each service&apos;s server-local date.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <PipelineFlow :stages="pipelineStages" />
      </CardContent>
    </Card>

    <!-- Metrics Grid -->
    <div class="grid gap-3 md:grid-cols-2 lg:grid-cols-4">
      <MetricCard
        title="Items Crawled"
        :value="metrics.contentToday"
        subtitle="URLs/pages today (crawler)"
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
        title="Active Channels"
        :value="`${metrics.activeRoutes}/${metrics.totalRoutes}`"
        subtitle="channels enabled"
        :icon="GitBranch"
      />
    </div>

    <!-- Intelligence Overview -->
    <Card v-if="intelligenceOverview || intelligenceLoading">
      <CardHeader class="pb-3">
        <CardTitle class="flex items-center gap-2 text-sm font-medium uppercase tracking-wider text-muted-foreground">
          <Brain class="h-4 w-4" />
          Intelligence Overview
        </CardTitle>
        <CardDescription class="text-xs">
          Content classification insights across all indexed documents
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div
          v-if="intelligenceLoading"
          class="text-center py-4 text-xs text-muted-foreground font-mono"
        >
          Loading intelligence data...
        </div>
        <div
          v-else-if="intelligenceOverview"
          class="grid gap-6 md:grid-cols-2 lg:grid-cols-4"
        >
          <!-- Total Documents -->
          <div class="space-y-0.5">
            <p class="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
              Total Documents
            </p>
            <p class="text-2xl font-semibold font-mono tracking-tight">
              {{ intelligenceOverview.total_documents.toLocaleString() }}
            </p>
          </div>

          <!-- Crime Related -->
          <div class="space-y-0.5">
            <p class="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
              Crime Related
            </p>
            <p class="text-2xl font-semibold font-mono tracking-tight text-red-500">
              {{ crimePercentage }}%
            </p>
            <p class="text-[10px] font-mono text-muted-foreground">
              {{ intelligenceOverview.total_crime_related.toLocaleString() }} articles
            </p>
          </div>

          <!-- Top Crime Types -->
          <div class="space-y-1.5">
            <p class="text-[10px] font-mono uppercase tracking-widest text-muted-foreground flex items-center gap-1">
              <AlertTriangle class="h-3 w-3" />
              Top Crime Types
            </p>
            <div class="flex flex-wrap gap-1">
              <Badge
                v-for="type in intelligenceOverview.top_crime_types?.slice(0, 3)"
                :key="type"
                variant="secondary"
              >
                {{ type.replace(/_/g, ' ') }}
              </Badge>
              <span
                v-if="!intelligenceOverview.top_crime_types?.length"
                class="text-xs text-muted-foreground font-mono"
              >None</span>
            </div>
          </div>

          <!-- Top Cities -->
          <div class="space-y-1.5">
            <p class="text-[10px] font-mono uppercase tracking-widest text-muted-foreground flex items-center gap-1">
              <MapPin class="h-3 w-3" />
              Top Cities
            </p>
            <div class="flex flex-wrap gap-1">
              <Badge
                v-for="city in intelligenceOverview.top_cities?.slice(0, 3)"
                :key="city"
                variant="outline"
              >
                {{ city }}
              </Badge>
              <span
                v-if="!intelligenceOverview.top_cities?.length"
                class="text-xs text-muted-foreground font-mono"
              >None</span>
            </div>
          </div>
        </div>

        <!-- Quality Distribution Bar -->
        <div
          v-if="intelligenceOverview?.quality_distribution"
          class="mt-4 pt-4 border-t"
        >
          <p class="text-[10px] font-mono uppercase tracking-widest text-muted-foreground mb-2">
            Quality Distribution
          </p>
          <div class="flex h-2 rounded-sm overflow-hidden bg-muted">
            <div
              class="bg-green-500"
              :style="{ width: `${(intelligenceOverview.quality_distribution.high / intelligenceOverview.total_documents) * 100}%` }"
              :title="`High: ${intelligenceOverview.quality_distribution.high}`"
            />
            <div
              class="bg-amber-500"
              :style="{ width: `${(intelligenceOverview.quality_distribution.medium / intelligenceOverview.total_documents) * 100}%` }"
              :title="`Medium: ${intelligenceOverview.quality_distribution.medium}`"
            />
            <div
              class="bg-red-500"
              :style="{ width: `${(intelligenceOverview.quality_distribution.low / intelligenceOverview.total_documents) * 100}%` }"
              :title="`Low: ${intelligenceOverview.quality_distribution.low}`"
            />
          </div>
          <div class="flex justify-between text-[10px] font-mono text-muted-foreground mt-1">
            <span>High ({{ intelligenceOverview.quality_distribution.high }})</span>
            <span>Medium ({{ intelligenceOverview.quality_distribution.medium }})</span>
            <span>Low ({{ intelligenceOverview.quality_distribution.low }})</span>
          </div>
        </div>

        <!-- Link to detailed views -->
        <div class="mt-4 pt-4 border-t flex gap-2">
          <Button
            variant="outline"
            size="xs"
            class="font-mono"
            @click="router.push('/intelligence/crime')"
          >
            Crime Details
            <ArrowRight class="ml-1 h-3 w-3" />
          </Button>
          <Button
            variant="outline"
            size="xs"
            class="font-mono"
            @click="router.push('/intelligence/location')"
          >
            Location Details
            <ArrowRight class="ml-1 h-3 w-3" />
          </Button>
        </div>
      </CardContent>
    </Card>

    <!-- Jobs Summary -->
    <Card>
      <CardHeader class="flex flex-row items-center justify-between pb-3">
        <div>
          <CardTitle class="flex items-center gap-2 text-sm font-medium uppercase tracking-wider text-muted-foreground">
            <Briefcase class="h-4 w-4" />
            Crawl Jobs
          </CardTitle>
          <CardDescription class="text-xs">
            Current status of all crawler jobs
          </CardDescription>
        </div>
        <Button
          variant="outline"
          size="xs"
          class="font-mono"
          @click="goToJobs"
        >
          View All
          <ArrowRight class="ml-1 h-3 w-3" />
        </Button>
      </CardHeader>
      <CardContent>
        <JobStatsCard compact />
      </CardContent>
    </Card>

    <!-- Bottom Row: Health + Quick Actions -->
    <div class="grid gap-3 md:grid-cols-3">
      <!-- System Health -->
      <Card class="md:col-span-2">
        <CardHeader class="pb-3">
          <CardTitle class="text-sm font-medium uppercase tracking-wider text-muted-foreground">
            System Health
          </CardTitle>
          <CardDescription class="text-xs">
            Status of all pipeline services
          </CardDescription>
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
