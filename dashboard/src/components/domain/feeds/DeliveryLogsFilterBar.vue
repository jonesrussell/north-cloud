<script setup lang="ts">
import { X } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import type { ActiveChannel } from '@/types/publisher'
import type { PublishHistoryTableFilters } from '@/composables'

defineProps<{
  filters: PublishHistoryTableFilters
  hasActiveFilters: boolean
  activeFilterCount: number
  channels?: ActiveChannel[]
}>()

defineEmits<{
  (e: 'update:channel_name', value: string): void
  (e: 'clear-filters'): void
}>()

function formatChannelName(channel: ActiveChannel): string {
  if (channel.name && channel.name !== channel.redis_channel) {
    return `${channel.name} (${channel.redis_channel})`
  }
  return channel.redis_channel
}
</script>

<template>
  <div class="flex flex-col gap-3 sm:flex-row sm:items-center">
    <div
      v-if="channels && channels.length > 0"
      class="sm:w-64"
    >
      <select
        :value="filters.channel_name || ''"
        class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
        @change="$emit('update:channel_name', ($event.target as HTMLSelectElement).value || '')"
      >
        <option value="">
          All Channels
        </option>
        <option
          v-for="channel in channels"
          :key="channel.redis_channel"
          :value="channel.redis_channel"
        >
          {{ formatChannelName(channel) }}
        </option>
      </select>
    </div>

    <Button
      v-if="hasActiveFilters"
      variant="outline"
      size="sm"
      class="shrink-0"
      @click="$emit('clear-filters')"
    >
      <X class="mr-1 h-4 w-4" />
      Clear ({{ activeFilterCount }})
    </Button>
  </div>
</template>
