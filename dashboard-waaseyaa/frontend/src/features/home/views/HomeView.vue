<script setup lang="ts">
import { useHomeStats } from '@/features/home/composables/useHomeStats'
import StatCard from '@/features/home/components/StatCard.vue'
import QuickActions from '@/features/home/components/QuickActions.vue'
import GrafanaEmbed from '@/shared/components/GrafanaEmbed.vue'
import LoadingSkeleton from '@/shared/components/LoadingSkeleton.vue'
import ErrorBanner from '@/shared/components/ErrorBanner.vue'

const { sourceCount, runningJobs, pendingReview, channelCount } = useHomeStats()

interface StatDef {
  label: string
  stat: typeof sourceCount
}

const stats: StatDef[] = [
  { label: 'Active Sources', stat: sourceCount },
  { label: 'Running Jobs', stat: runningJobs },
  { label: 'Pending Review', stat: pendingReview },
  { label: 'Channels', stat: channelCount },
]
</script>

<template>
  <div class="space-y-8">
    <h1 class="text-2xl font-bold text-slate-100">Pipeline Overview</h1>

    <!-- Stat Cards Grid -->
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
      <div v-for="s in stats" :key="s.label">
        <LoadingSkeleton v-if="s.stat.value.isLoading" :lines="2" />
        <ErrorBanner
          v-else-if="s.stat.value.isError"
          :message="`Failed to load ${s.label.toLowerCase()}`"
          @retry="s.stat.value.refetch()"
        />
        <StatCard
          v-else
          :label="s.label"
          :value="s.stat.value.value"
        />
      </div>
    </div>

    <!-- Grafana Panel -->
    <section>
      <h2 class="text-lg font-semibold text-slate-200 mb-3">Pipeline Throughput</h2>
      <GrafanaEmbed panel-id="pipeline-throughput" height="350px" />
    </section>

    <!-- Quick Actions -->
    <section>
      <h2 class="text-lg font-semibold text-slate-200 mb-3">Quick Actions</h2>
      <QuickActions />
    </section>
  </div>
</template>
