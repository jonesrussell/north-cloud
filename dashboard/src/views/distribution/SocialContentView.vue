<script setup lang="ts">
import { Loader2, FileText } from 'lucide-vue-next'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import ContentFilterBar from '@/components/domain/social-publishing/ContentFilterBar.vue'
import ContentTable from '@/components/domain/social-publishing/ContentTable.vue'
import { useContentTable } from '@/features/social-publishing'

const contentTable = useContentTable()

function onStatusChange(value: string | undefined) {
  contentTable.setFilter('status', value)
}

function onTypeChange(value: string | undefined) {
  contentTable.setFilter('type', value)
}

function onRetry() {
  contentTable.refetch()
}
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-3xl font-bold tracking-tight">
        Social Content
      </h1>
      <p class="text-muted-foreground">
        Published and scheduled content with delivery tracking
      </p>
    </div>

    <div
      v-if="contentTable.isLoading.value && contentTable.items.value.length === 0"
      class="flex items-center justify-center py-12"
    >
      <Loader2 class="h-8 w-8 animate-spin text-muted-foreground" />
    </div>

    <Card
      v-else-if="contentTable.error.value"
      class="border-destructive"
    >
      <CardContent class="pt-6">
        <p class="text-destructive">
          {{ contentTable.error.value?.message || 'Unable to load content.' }}
        </p>
      </CardContent>
    </Card>

    <Card v-else-if="contentTable.items.value.length === 0 && !contentTable.hasActiveFilters.value">
      <CardContent class="flex flex-col items-center justify-center py-12">
        <FileText class="h-12 w-12 text-muted-foreground mb-4" />
        <h3 class="text-lg font-medium mb-2">
          No social content yet
        </h3>
        <p class="text-muted-foreground">
          Content will appear here after you publish using the Publish page.
        </p>
      </CardContent>
    </Card>

    <template v-else>
      <Card>
        <CardHeader class="pb-4">
          <CardTitle class="text-base">
            Filter Content
          </CardTitle>
        </CardHeader>
        <CardContent>
          <ContentFilterBar
            :filters="contentTable.filters.value"
            :has-active-filters="contentTable.hasActiveFilters.value"
            :active-filter-count="contentTable.activeFilterCount.value"
            @update:status="onStatusChange"
            @update:type="onTypeChange"
            @clear-filters="contentTable.clearFilters"
          />
        </CardContent>
      </Card>

      <Card>
        <CardContent class="p-0">
          <ContentTable
            :items="contentTable.items.value"
            :total="contentTable.total.value"
            :is-loading="contentTable.isLoading.value"
            :page="contentTable.page.value"
            :page-size="contentTable.pageSize.value"
            :total-pages="contentTable.totalPages.value"
            :allowed-page-sizes="contentTable.allowedPageSizes"
            :sort-by="contentTable.sortBy.value"
            :sort-order="contentTable.sortOrder.value"
            :has-active-filters="contentTable.hasActiveFilters.value"
            :on-sort="contentTable.toggleSort"
            :on-page-change="contentTable.setPage"
            :on-page-size-change="contentTable.setPageSize"
            :on-clear-filters="contentTable.clearFilters"
            :on-retry="onRetry"
          />
        </CardContent>
      </Card>
    </template>
  </div>
</template>
