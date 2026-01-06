<script setup lang="ts">
import { computed } from 'vue'
import { ArrowRight } from 'lucide-vue-next'
import { cn } from '@/lib/utils'

interface PipelineStage {
  name: string
  count: number
  change?: number // percentage change
  status?: 'healthy' | 'warning' | 'error'
}

interface Props {
  stages: PipelineStage[]
}

const props = defineProps<Props>()

const getChangeClass = (change?: number) => {
  if (!change) return 'text-muted-foreground'
  return change > 0 ? 'text-green-500' : 'text-red-500'
}

const formatChange = (change?: number) => {
  if (!change) return ''
  return change > 0 ? `↑${change}%` : `↓${Math.abs(change)}%`
}

const getStatusColor = (status?: string) => {
  switch (status) {
    case 'healthy':
      return 'border-green-500/50 bg-green-500/5'
    case 'warning':
      return 'border-yellow-500/50 bg-yellow-500/5'
    case 'error':
      return 'border-red-500/50 bg-red-500/5'
    default:
      return 'border-border'
  }
}
</script>

<template>
  <div class="flex items-center justify-between gap-2 overflow-x-auto pb-2">
    <template v-for="(stage, index) in stages" :key="stage.name">
      <!-- Stage card -->
      <div
        :class="
          cn(
            'flex-1 min-w-[120px] rounded-lg border-2 p-4 text-center transition-all',
            getStatusColor(stage.status)
          )
        "
      >
        <div class="text-2xl font-bold text-foreground">
          {{ stage.count.toLocaleString() }}
        </div>
        <div class="text-sm font-medium text-muted-foreground mt-1">
          {{ stage.name }}
        </div>
        <div v-if="stage.change !== undefined" :class="cn('text-xs mt-1', getChangeClass(stage.change))">
          {{ formatChange(stage.change) }}
        </div>
      </div>

      <!-- Arrow between stages -->
      <ArrowRight
        v-if="index < stages.length - 1"
        class="h-5 w-5 text-muted-foreground shrink-0"
      />
    </template>
  </div>
</template>
