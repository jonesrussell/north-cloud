<script setup lang="ts">
import { computed } from 'vue'
import { Activity, CheckCircle2, XCircle, Clock, TrendingUp } from 'lucide-vue-next'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { useJobsStore } from '@/stores/jobs'

interface Props {
  compact?: boolean
}

withDefaults(defineProps<Props>(), {
  compact: false,
})

const jobsStore = useJobsStore()

const stats = computed(() => [
  {
    label: 'Total Jobs',
    value: jobsStore.items.length,
    icon: Activity,
    color: 'text-blue-500',
    bgColor: 'bg-blue-500/10',
  },
  {
    label: 'Active',
    value: jobsStore.activeJobsCount,
    icon: TrendingUp,
    color: 'text-green-500',
    bgColor: 'bg-green-500/10',
  },
  {
    label: 'Completed',
    value: jobsStore.statusCounts.completed,
    icon: CheckCircle2,
    color: 'text-emerald-500',
    bgColor: 'bg-emerald-500/10',
  },
  {
    label: 'Failed',
    value: jobsStore.failedJobsCount,
    icon: XCircle,
    color: jobsStore.failedJobsCount > 0 ? 'text-red-500' : 'text-muted-foreground',
    bgColor: jobsStore.failedJobsCount > 0 ? 'bg-red-500/10' : 'bg-muted',
  },
  {
    label: 'Paused',
    value: jobsStore.statusCounts.paused,
    icon: Clock,
    color: 'text-yellow-500',
    bgColor: 'bg-yellow-500/10',
  },
])
</script>

<template>
  <div
    v-if="compact"
    class="flex flex-wrap gap-4"
  >
    <div
      v-for="stat in stats"
      :key="stat.label"
      class="flex items-center gap-2 rounded-lg border px-3 py-2"
    >
      <component
        :is="stat.icon"
        :class="['h-4 w-4', stat.color]"
      />
      <span class="text-sm font-medium">{{ stat.value }}</span>
      <span class="text-xs text-muted-foreground">{{ stat.label }}</span>
    </div>
  </div>

  <div
    v-else
    class="grid gap-4 md:grid-cols-5"
  >
    <Card
      v-for="stat in stats"
      :key="stat.label"
    >
      <CardHeader class="flex flex-row items-center justify-between pb-2">
        <CardTitle class="text-sm font-medium text-muted-foreground">
          {{ stat.label }}
        </CardTitle>
        <div :class="['rounded-md p-2', stat.bgColor]">
          <component
            :is="stat.icon"
            :class="['h-4 w-4', stat.color]"
          />
        </div>
      </CardHeader>
      <CardContent>
        <div class="text-2xl font-bold">
          {{ stat.value }}
        </div>
      </CardContent>
    </Card>
  </div>
</template>
