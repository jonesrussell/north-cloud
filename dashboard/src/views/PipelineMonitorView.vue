<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import {
  Download,
  Share2,
  BarChart3,
  GitBranch,
  Briefcase,
  ArrowRight,
  Brain,
} from 'lucide-vue-next'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { PipelineFlow, MetricCard, HealthBadge, QuickActions } from '@/components/pipeline'
import { JobStatsCard } from '@/components/domain/jobs'
import { Skeleton } from '@/components/ui/skeleton'
import { useHealthStore, useMetricsStore } from '@/stores'
import { indexManagerApi } from '@/api/client'
import { DEFAULT_QUICK_ACTIONS } from '@/types/metrics'

const router = useRouter()
const healthStore = useHealthStore()
const metricsStore = useMetricsStore()

const intelligenceTotal = ref(0)
const intelligenceQuality = ref({ high: 0, medium: 0, low: 0 })
const intelligenceLoading = ref(true)
const intelligenceHasLoaded = ref(false)

async function fetchIntelligence() {
  intelligenceLoading.value = true
  try {
    const res = await indexManagerApi.aggregations.getOverview()
    intelligenceTotal.value = res.data?.total_documents ?? 0
    const q = res.data?.quality_distribution
    intelligenceQuality.value = {
      high: Math.max(0, Number(q?.high) || 0),
      medium: Math.max(0, Number(q?.medium) || 0),
      low: Math.max(0, Number(q?.low) || 0),
    }
  } catch {
    intelligenceTotal.value = 0
    intelligenceQuality.value = { high: 0, medium: 0, low: 0 }
  } finally {
    intelligenceLoading.value = false
    intelligenceHasLoaded.value = true
  }
}

const qualityBarWidths = computed(() => {
  const { high, medium, low } = intelligenceQuality.value
  const total = high + medium + low
  if (total === 0) return { high: 0, medium: 0, low: 0 }
  return {
    high: (high / total) * 100,
    medium: (medium / total) * 100,
    low: (low / total) * 100,
  }
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
  fetchIntelligence()
})

onUnmounted(() => {
  healthStore.stopPolling()
  metricsStore.stopPolling()
})

function goToJobs() {
  router.push({ name: 'intake-jobs' })
}

function goToIntelligence() {
  router.push('/intelligence')
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

    <!-- Content Intelligence (compact) -->
    <Card>
      <CardHeader class="pb-3">
        <CardTitle class="flex items-center gap-2 text-sm font-medium uppercase tracking-wider text-muted-foreground">
          <Brain class="h-4 w-4" />
          Content Intelligence
        </CardTitle>
        <CardDescription class="text-xs">
          Classification insights across all indexed documents
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div
          v-if="!intelligenceHasLoaded || intelligenceLoading"
          class="space-y-2"
        >
          <Skeleton class="h-6 w-20" />
          <Skeleton class="h-1.5 w-full rounded-sm" />
        </div>
        <template v-else>
          <div class="space-y-2">
            <p class="text-lg font-semibold font-mono tracking-tight">
              {{ intelligenceTotal.toLocaleString() }}
            </p>
            <div class="flex rounded-sm overflow-hidden bg-muted h-1.5">
              <div
                class="bg-green-500 shrink-0 transition-[width]"
                :style="{ width: `${qualityBarWidths.high}%` }"
                :title="`High: ${intelligenceQuality.high}`"
              />
              <div
                class="bg-amber-500 shrink-0 transition-[width]"
                :style="{ width: `${qualityBarWidths.medium}%` }"
                :title="`Medium: ${intelligenceQuality.medium}`"
              />
              <div
                class="bg-red-500 shrink-0 transition-[width]"
                :style="{ width: `${qualityBarWidths.low}%` }"
                :title="`Low: ${intelligenceQuality.low}`"
              />
            </div>
          </div>
          <div class="mt-4 pt-3 border-t">
            <Button
              variant="outline"
              size="xs"
              class="font-mono w-full justify-between"
              @click="goToIntelligence()"
            >
              View Intelligence
              <ArrowRight class="ml-1 h-3 w-3" />
            </Button>
          </div>
        </template>
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
