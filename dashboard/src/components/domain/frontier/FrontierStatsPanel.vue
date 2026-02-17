<script setup lang="ts">
import { Clock, Loader2, CheckCircle, XCircle, Skull } from 'lucide-vue-next'
import { Card, CardContent } from '@/components/ui/card'
import type { FrontierStats } from '@/features/intake/api/frontier'

defineProps<{
  stats: FrontierStats | null
  isLoading: boolean
}>()

const statCards = [
  { key: 'total_pending', label: 'Pending', icon: Clock, color: 'text-blue-500' },
  { key: 'total_fetching', label: 'Fetching', icon: Loader2, color: 'text-yellow-500' },
  { key: 'total_fetched', label: 'Fetched', icon: CheckCircle, color: 'text-green-500' },
  { key: 'total_failed', label: 'Failed', icon: XCircle, color: 'text-red-500' },
  { key: 'total_dead', label: 'Dead', icon: Skull, color: 'text-muted-foreground' },
] as const
</script>

<template>
  <div class="grid gap-4 sm:grid-cols-5">
    <Card
      v-for="stat in statCards"
      :key="stat.key"
    >
      <CardContent class="flex items-center gap-3 pt-6">
        <component
          :is="stat.icon"
          :class="['h-5 w-5 shrink-0', stat.color]"
        />
        <div class="min-w-0">
          <p class="text-2xl font-bold tabular-nums">
            {{ isLoading ? '...' : (stats?.[stat.key] ?? 0).toLocaleString() }}
          </p>
          <p class="text-xs text-muted-foreground">
            {{ stat.label }}
          </p>
        </div>
      </CardContent>
    </Card>
  </div>
</template>
