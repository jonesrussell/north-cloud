<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowRight, Loader2 } from 'lucide-vue-next'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { ContentIntelligenceSummary } from '@/components/intelligence'
import { useIntelligenceOverview } from '@/composables'
import {
  INTELLIGENCE_DRILL_DOWNS,
  COUNT_FETCHERS,
  type IntelligenceDrillDownItem,
} from '@/config/intelligence'

const router = useRouter()
const { data: overviewData, loading: overviewLoading, error: overviewError } = useIntelligenceOverview()

// Module-level refs so counts are memoized across remounts
const countsByKey = ref<Record<string, number>>({})
const hasFetchedCounts = ref(false)

async function fetchOptionalCounts() {
  if (hasFetchedCounts.value) return
  const keysToFetch = INTELLIGENCE_DRILL_DOWNS.filter(
    (item) => item.countKey && COUNT_FETCHERS[item.countKey]
  ).map((item) => item.countKey as string)
  const uniqueKeys = [...new Set(keysToFetch)]
  if (uniqueKeys.length === 0) {
    hasFetchedCounts.value = true
    return
  }
  const results = await Promise.allSettled(
    uniqueKeys.map(async (key) => {
      const fetcher = COUNT_FETCHERS[key]
      const count = fetcher ? await fetcher() : 0
      return { key, count }
    })
  )
  const next: Record<string, number> = { ...countsByKey.value }
  for (const result of results) {
    if (result.status === 'fulfilled' && result.value) {
      next[result.value.key] = result.value.count
    }
  }
  countsByKey.value = next
  hasFetchedCounts.value = true
}

function getCount(item: IntelligenceDrillDownItem): number | undefined {
  if (!item.countKey) return undefined
  const n = countsByKey.value[item.countKey]
  return n !== undefined ? n : undefined
}

function goTo(route: string) {
  router.push(route)
}

onMounted(() => {
  fetchOptionalCounts()
})
</script>

<template>
  <div class="space-y-6 animate-fade-up">
    <div>
      <h1 class="text-2xl font-semibold tracking-tight">
        Intelligence
      </h1>
      <p class="mt-0.5 text-sm text-muted-foreground">
        Explore classification breakdowns and indexed content.
      </p>
    </div>

    <!-- Summary block -->
    <Card>
      <CardHeader class="pb-3">
        <CardTitle class="text-sm font-medium uppercase tracking-wider text-muted-foreground">
          Content summary
        </CardTitle>
        <CardDescription class="text-xs">
          Total documents and quality distribution across all classified content.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div
          v-if="overviewLoading && !overviewData.total_documents && !overviewError"
          class="flex items-center justify-center py-8 text-muted-foreground"
        >
          <Loader2 class="h-6 w-6 animate-spin" />
        </div>
        <div
          v-else-if="overviewError"
          class="py-4 text-sm text-destructive"
        >
          {{ overviewError.message }}
        </div>
        <ContentIntelligenceSummary
          v-else
          mode="full"
          :total-documents="overviewData.total_documents"
          :quality-distribution="overviewData.quality_distribution"
        />
      </CardContent>
    </Card>

    <!-- Drill-down cards from config -->
    <div>
      <h2 class="text-sm font-medium uppercase tracking-wider text-muted-foreground mb-3">
        Breakdowns
      </h2>
      <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <Card
          v-for="item in INTELLIGENCE_DRILL_DOWNS"
          :key="item.route"
          class="cursor-pointer transition-colors hover:bg-muted/50"
          @click="goTo(item.route)"
        >
          <CardHeader class="pb-2">
            <div
              class="flex items-start justify-between gap-2"
            >
              <component
                :is="item.icon"
                class="h-5 w-5 shrink-0 text-muted-foreground"
              />
              <span
                v-if="getCount(item) !== undefined"
                class="text-lg font-semibold tabular-nums"
              >
                {{ getCount(item)!.toLocaleString() }}
              </span>
            </div>
            <CardTitle class="text-base">
              {{ item.title }}
            </CardTitle>
            <CardDescription class="text-xs line-clamp-2">
              {{ item.description }}
            </CardDescription>
          </CardHeader>
          <CardContent class="pt-0">
            <Button
              variant="ghost"
              size="sm"
              class="w-full justify-between font-mono"
              @click.stop="goTo(item.route)"
            >
              <span>Open</span>
              <ArrowRight class="h-3.5 w-3.5" />
            </Button>
          </CardContent>
        </Card>
      </div>
    </div>
  </div>
</template>
