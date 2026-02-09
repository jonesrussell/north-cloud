<script setup lang="ts">
import { X, RefreshCw, Radio } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import type { ActiveChannel } from '@/types/publisher'
import type { PublishHistoryFilters } from '@/composables/usePublishHistory'

interface Props {
  channels: ActiveChannel[]
  filters: PublishHistoryFilters
  hasActiveFilters: boolean
  activeFilterCount: number
  isPolling: boolean
  isPaused: boolean
  loading?: boolean
}

const props = defineProps<Props>()

const emit = defineEmits<{
  'update:channel': [channelName: string | undefined]
  'clear-filters': []
  'refresh': []
}>()

function handleChannelChange(event: Event) {
  const target = event.target as HTMLSelectElement
  emit('update:channel', target.value || undefined)
}

function formatChannelName(channel: ActiveChannel): string {
  // Use display name if available, otherwise clean up redis_channel name
  if (channel.name && channel.name !== channel.redis_channel) {
    return `${channel.name} (${channel.redis_channel})`
  }
  return channel.redis_channel
}
</script>

<template>
  <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center">
      <!-- Channel Filter -->
      <div class="sm:w-64">
        <select
          :value="props.filters.channel_name || ''"
          class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          @change="handleChannelChange"
        >
          <option value="">
            All Channels
          </option>
          <option
            v-for="channel in props.channels"
            :key="channel.redis_channel"
            :value="channel.redis_channel"
          >
            {{ formatChannelName(channel) }}
          </option>
        </select>
      </div>

      <!-- Clear Filters Button -->
      <Button
        v-if="props.hasActiveFilters"
        variant="outline"
        size="sm"
        class="shrink-0"
        @click="emit('clear-filters')"
      >
        <X class="mr-1 h-4 w-4" />
        Clear ({{ props.activeFilterCount }})
      </Button>
    </div>

    <div class="flex items-center gap-2">
      <!-- Polling Indicator -->
      <div
        v-if="props.isPolling"
        class="flex items-center gap-1.5 text-xs text-muted-foreground"
      >
        <Radio
          class="h-3 w-3"
          :class="props.isPaused ? 'text-yellow-500' : 'text-green-500 animate-pulse'"
        />
        <span>{{ props.isPaused ? 'Paused' : 'Live' }}</span>
      </div>

      <!-- Refresh Button -->
      <Button
        variant="outline"
        size="sm"
        :disabled="props.loading"
        @click="emit('refresh')"
      >
        <RefreshCw
          class="mr-2 h-4 w-4"
          :class="{ 'animate-spin': props.loading }"
        />
        Refresh
      </Button>
    </div>
  </div>
</template>
