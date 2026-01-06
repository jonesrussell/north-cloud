<script setup lang="ts">
import { computed, type Component } from 'vue'
import { TrendingUp, TrendingDown } from 'lucide-vue-next'
import { cn } from '@/lib/utils'
import { Card, CardContent } from '@/components/ui/card'

interface Props {
  title: string
  value: string | number
  subtitle?: string
  change?: number
  icon?: Component
  trend?: 'up' | 'down' | 'neutral'
}

const props = defineProps<Props>()

const trendIcon = computed(() => {
  if (props.trend === 'up') return TrendingUp
  if (props.trend === 'down') return TrendingDown
  return null
})

const trendClass = computed(() => {
  if (props.trend === 'up') return 'text-green-500'
  if (props.trend === 'down') return 'text-red-500'
  return 'text-muted-foreground'
})
</script>

<template>
  <Card>
    <CardContent class="p-6">
      <div class="flex items-start justify-between">
        <div class="space-y-1">
          <p class="text-sm font-medium text-muted-foreground">
            {{ title }}
          </p>
          <p class="text-2xl font-bold">
            {{ value }}
          </p>
          <p
            v-if="subtitle"
            class="text-xs text-muted-foreground"
          >
            {{ subtitle }}
          </p>
        </div>
        <div
          v-if="icon"
          class="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10"
        >
          <component
            :is="icon"
            class="h-5 w-5 text-primary"
          />
        </div>
      </div>

      <div
        v-if="change !== undefined"
        class="mt-4 flex items-center gap-1"
      >
        <component
          :is="trendIcon"
          v-if="trendIcon"
          :class="cn('h-4 w-4', trendClass)"
        />
        <span :class="cn('text-sm', trendClass)">
          {{ change > 0 ? '+' : '' }}{{ change }}%
        </span>
        <span class="text-xs text-muted-foreground">vs yesterday</span>
      </div>
    </CardContent>
  </Card>
</template>
