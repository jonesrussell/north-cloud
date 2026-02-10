<script setup lang="ts">
import { computed } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { Loader2, ScrollText, RefreshCw } from 'lucide-vue-next'
import { publisherApi } from '@/api/client'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import {
  DeliveryLogsFilterBar,
  DeliveryLogsTable,
} from '@/components/domain/feeds'
import { usePublishHistoryTable } from '@/composables'

const table = usePublishHistoryTable()

const { data: channelsData } = useQuery({
  queryKey: ['publisher', 'active-channels'],
  queryFn: async () => {
    const res = await publisherApi.stats.activeChannels()
    return res.data
  },
})

const channels = computed(() => channelsData.value?.channels ?? [])

function onChannelChange(value: string) {
  table.setFilter('channel_name', value || undefined)
}
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-3xl font-bold tracking-tight">
          Delivery Logs
        </h1>
        <p class="text-muted-foreground">
          Track article publication to channels
        </p>
      </div>
      <Button
        variant="outline"
        @click="table.refetch"
      >
        <RefreshCw class="mr-2 h-4 w-4" />
        Refresh
      </Button>
    </div>

    <div
      v-if="table.isLoading.value && table.items.value.length === 0"
      class="flex items-center justify-center py-12"
    >
      <Loader2 class="h-8 w-8 animate-spin text-muted-foreground" />
    </div>

    <Card
      v-else-if="table.error.value"
      class="border-destructive"
    >
      <CardContent class="pt-6">
        <p class="text-destructive">
          {{ table.error.value?.message || 'Unable to load delivery logs.' }}
        </p>
      </CardContent>
    </Card>

    <Card
      v-else-if="table.items.value.length === 0 && !table.hasActiveFilters.value"
      class="border-dashed"
    >
      <CardContent class="flex flex-col items-center justify-center py-12">
        <ScrollText class="h-12 w-12 text-muted-foreground mb-4" />
        <h3 class="text-lg font-medium mb-2">
          No delivery logs
        </h3>
        <p class="text-muted-foreground">
          Logs will appear here when articles are published.
        </p>
      </CardContent>
    </Card>

    <template v-else>
      <Card>
        <CardHeader class="pb-4">
          <CardTitle class="text-base">
            Filter Logs
          </CardTitle>
        </CardHeader>
        <CardContent>
          <DeliveryLogsFilterBar
            :filters="table.filters.value"
            :has-active-filters="table.hasActiveFilters.value"
            :active-filter-count="table.activeFilterCount.value"
            :channels="channels"
            @update:channel_name="onChannelChange"
            @clear-filters="table.clearFilters"
          />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Delivery Logs</CardTitle>
        </CardHeader>
        <CardContent class="p-0">
          <DeliveryLogsTable
            :items="table.items.value"
            :total="table.total.value"
            :is-loading="table.isLoading.value"
            :page="table.page.value"
            :page-size="table.pageSize.value"
            :total-pages="table.totalPages.value"
            :allowed-page-sizes="table.allowedPageSizes"
            :has-active-filters="table.hasActiveFilters.value"
            :on-page-change="table.setPage"
            :on-page-size-change="table.setPageSize"
            :on-clear-filters="table.clearFilters"
          />
        </CardContent>
      </Card>
    </template>
  </div>
</template>
