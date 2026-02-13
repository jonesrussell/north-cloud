<script setup lang="ts">
import { Loader2, RefreshCw } from 'lucide-vue-next'
import { usePipelineHealth } from '@/features/intelligence/composables/usePipelineHealth'
import ProblemsBanner from '@/features/intelligence/problems/ProblemsBanner.vue'
import PipelineKPIs from '@/features/intelligence/components/PipelineKPIs.vue'
import SourceHealthTable from '@/features/intelligence/components/SourceHealthTable.vue'
import ContentSummaryCards from '@/features/intelligence/components/ContentSummaryCards.vue'
import ContentTypeDriftPanel from '@/features/intelligence/components/ContentTypeDriftPanel.vue'
import SuspectedMisclassificationTable from '@/features/intelligence/components/SuspectedMisclassificationTable.vue'
import PublishabilityFunnel from '@/features/intelligence/components/PublishabilityFunnel.vue'

const { metrics, loading, problems, refresh } = usePipelineHealth()
</script>

<template>
  <div class="space-y-6 animate-fade-up">
    <!-- Header -->
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-semibold tracking-tight">
          Intelligence
        </h1>
        <p class="mt-0.5 text-sm text-muted-foreground">
          Pipeline health and content intelligence.
        </p>
      </div>
      <button
        class="inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-medium bg-muted hover:bg-muted/80 transition-colors"
        :disabled="loading"
        @click="refresh"
      >
        <RefreshCw
          class="h-3.5 w-3.5"
          :class="{ 'animate-spin': loading }"
        />
        Refresh
      </button>
    </div>

    <!-- Loading state -->
    <div
      v-if="loading && !metrics.indexes"
      class="flex items-center justify-center py-16"
    >
      <Loader2 class="h-6 w-6 animate-spin text-muted-foreground" />
    </div>

    <template v-else>
      <!-- Problems Banner -->
      <ProblemsBanner :problems="problems" />

      <!-- Pipeline KPIs -->
      <PipelineKPIs :metrics="metrics" />

      <!-- Source Health Table -->
      <div>
        <h2 class="text-sm font-medium uppercase tracking-wider text-muted-foreground mb-3">
          Source Health
        </h2>
        <SourceHealthTable :sources="metrics.indexes?.sources ?? []" />
      </div>

      <!-- Content Intelligence -->
      <div>
        <h2 class="text-sm font-medium uppercase tracking-wider text-muted-foreground mb-3">
          Content Intelligence
        </h2>
        <ContentSummaryCards />
      </div>

      <!-- Publishability funnel -->
      <div>
        <h2 class="text-sm font-medium uppercase tracking-wider text-muted-foreground mb-3">
          Publishability Funnel
        </h2>
        <PublishabilityFunnel />
      </div>

      <!-- Content Type Drift (last 7 days) -->
      <div>
        <h2 class="text-sm font-medium uppercase tracking-wider text-muted-foreground mb-3">
          Content Type Drift
        </h2>
        <ContentTypeDriftPanel />
      </div>

      <!-- Recent misclassifications (24h) - one-click preset -->
      <div id="recent-misclassifications">
        <h2 class="text-sm font-medium uppercase tracking-wider text-muted-foreground mb-3">
          <a
            href="#recent-misclassifications"
            class="hover:underline"
          >Recent misclassifications (24h)</a>
        </h2>
        <SuspectedMisclassificationTable />
      </div>
    </template>
  </div>
</template>
