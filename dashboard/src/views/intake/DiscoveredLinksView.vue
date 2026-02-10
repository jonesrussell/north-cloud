<script setup lang="ts">
import { computed } from 'vue'
import { Loader2, Link, RefreshCw } from 'lucide-vue-next'
import { useQuery } from '@tanstack/vue-query'
import { sourcesApi } from '@/api/client'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { DiscoveredLinksFilterBar, DiscoveredLinksTable } from '@/components/domain/discovered-links'
import { useDiscoveredLinksTable } from '@/features/intake'
import { crawlerApi } from '@/api/client'

const table = useDiscoveredLinksTable()

const { data: sourcesData } = useQuery({
  queryKey: ['sources', 'list-dropdown'],
  queryFn: async () => {
    const res = await sourcesApi.list({ limit: 500, offset: 0 })
    return res.data
  },
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

async function deleteLink(id: string) {
  if (!confirm('Delete this discovered link?')) return
  try {
    await crawlerApi.discoveredLinks.delete(id)
    table.refetch()
  } catch (err) {
    console.error('Error deleting link:', err)
  }
}
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-3xl font-bold tracking-tight">
          Discovered Links
        </h1>
        <p class="text-muted-foreground">
          Links discovered during crawling awaiting processing
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
      v-if="table.isLoading.value && table.links.value.length === 0"
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
          {{ table.error.value?.message || 'Unable to load discovered links.' }}
        </p>
      </CardContent>
    </Card>

    <Card
      v-else-if="table.links.value.length === 0 && !table.hasActiveFilters.value"
      class="border-dashed"
    >
      <CardContent class="flex flex-col items-center justify-center py-12">
        <Link class="h-12 w-12 text-muted-foreground mb-4" />
        <h3 class="text-lg font-medium mb-2">
          No discovered links
        </h3>
        <p class="text-muted-foreground">
          Links discovered during crawling will appear here.
        </p>
      </CardContent>
    </Card>

    <template v-else>
      <Card>
        <CardHeader class="pb-4">
          <CardTitle class="text-base">
            Filter Links
          </CardTitle>
        </CardHeader>
        <CardContent>
          <DiscoveredLinksFilterBar
            :filters="table.filters.value"
            :has-active-filters="table.hasActiveFilters.value"
            :active-filter-count="table.activeFilterCount.value"
            :sources="sources"
            @update:search="onSearchChange"
            @update:status="onStatusChange"
            @update:source_id="onSourceChange"
            @clear-filters="table.clearFilters"
          />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Discovered Links</CardTitle>
        </CardHeader>
        <CardContent class="p-0">
          <DiscoveredLinksTable
            :links="table.links.value"
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
            :on-delete="deleteLink"
          />
        </CardContent>
      </Card>
    </template>
  </div>
</template>
