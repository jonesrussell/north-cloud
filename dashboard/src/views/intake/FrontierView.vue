<script setup lang="ts">
import { computed } from 'vue'
import { Loader2, Globe, RefreshCw } from 'lucide-vue-next'
import { useQuery } from '@tanstack/vue-query'
import { sourcesApi } from '@/api/client'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { FrontierFilterBar, FrontierTable, FrontierStatsPanel } from '@/components/domain/frontier'
import { useFrontierTable } from '@/features/intake'
import { fetchFrontierStats } from '@/features/intake/api/frontier'
import type { FrontierStats } from '@/features/intake/api/frontier'
import { crawlerApi } from '@/api/client'

const table = useFrontierTable()

const { data: sourcesData } = useQuery({
  queryKey: ['sources', 'list-dropdown'],
  queryFn: async () => {
    const res = await sourcesApi.list({ limit: 500, offset: 0 })
    return res.data
  },
})

const { data: statsData, isLoading: statsLoading, refetch: refetchStats } = useQuery<FrontierStats>({
  queryKey: ['frontier', 'stats'],
  queryFn: fetchFrontierStats,
  refetchInterval: 30_000,
})

const sources = computed(() => {
  const list = sourcesData.value?.sources ?? []
  return list.map((s: { id: string; name: string }) => ({ id: s.id, name: s.name || s.id }))
})

function onSearchChange(value: string) {
  table.setFilter('search', value || undefined)
}

function onStatusChange(value: string) {
  table.setFilter('status', value || undefined)
}

function onSourceChange(value: string) {
  table.setFilter('source_id', value || undefined)
}

function onHostChange(value: string) {
  table.setFilter('host', value || undefined)
}

function onOriginChange(value: string) {
  table.setFilter('origin', value || undefined)
}

function refreshAll() {
  table.refetch()
  refetchStats()
}

async function deleteUrl(id: string) {
  if (!confirm('Delete this URL from the frontier?')) return
  try {
    await crawlerApi.frontier.delete(id)
    table.refetch()
    refetchStats()
  } catch (err) {
    console.error('Error deleting frontier URL:', err)
  }
}
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-3xl font-bold tracking-tight">
          URL Frontier
        </h1>
        <p class="text-muted-foreground">
          URLs queued for fetching by the frontier workers
        </p>
      </div>
      <Button
        variant="outline"
        @click="refreshAll"
      >
        <RefreshCw class="mr-2 h-4 w-4" />
        Refresh
      </Button>
    </div>

    <FrontierStatsPanel
      :stats="statsData ?? null"
      :is-loading="statsLoading"
    />

    <div
      v-if="table.isLoading.value && table.urls.value.length === 0"
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
          {{ table.error.value?.message || 'Unable to load frontier URLs.' }}
        </p>
      </CardContent>
    </Card>

    <Card
      v-else-if="table.urls.value.length === 0 && !table.hasActiveFilters.value"
      class="border-dashed"
    >
      <CardContent class="flex flex-col items-center justify-center py-12">
        <Globe class="h-12 w-12 text-muted-foreground mb-4" />
        <h3 class="text-lg font-medium mb-2">
          No URLs in frontier
        </h3>
        <p class="text-muted-foreground">
          URLs discovered via feeds, sitemaps, or spiders will appear here.
        </p>
      </CardContent>
    </Card>

    <template v-else>
      <Card>
        <CardHeader class="pb-4">
          <CardTitle class="text-base">
            Filter URLs
          </CardTitle>
        </CardHeader>
        <CardContent>
          <FrontierFilterBar
            :filters="table.filters.value"
            :has-active-filters="table.hasActiveFilters.value"
            :active-filter-count="table.activeFilterCount.value"
            :sources="sources"
            @update:search="onSearchChange"
            @update:status="onStatusChange"
            @update:source_id="onSourceChange"
            @update:host="onHostChange"
            @update:origin="onOriginChange"
            @clear-filters="table.clearFilters"
          />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Frontier URLs</CardTitle>
        </CardHeader>
        <CardContent class="p-0">
          <FrontierTable
            :urls="table.urls.value"
            :total="table.total.value"
            :is-loading="table.isLoading.value"
            :page="table.page.value"
            :page-size="table.pageSize.value"
            :total-pages="table.totalPages.value"
            :allowed-page-sizes="table.allowedPageSizes"
            :sort-by="table.sortBy.value"
            :sort-order="table.sortOrder.value"
            :has-active-filters="table.hasActiveFilters.value"
            :on-sort="table.toggleSort"
            :on-page-change="table.setPage"
            :on-page-size-change="table.setPageSize"
            :on-clear-filters="table.clearFilters"
            :on-delete="deleteUrl"
          />
        </CardContent>
      </Card>
    </template>
  </div>
</template>
