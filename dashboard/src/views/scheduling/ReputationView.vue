<script setup lang="ts">
import { Loader2, Star } from 'lucide-vue-next'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { ReputationFilterBar, ReputationTable } from '@/components/domain/reputation'
import { useReputationTable } from '@/features/scheduling'

const reputation = useReputationTable()
const categoryOptions = ['news', 'blog', 'government', 'unknown']

function onSearchChange(value: string) {
  reputation.setFilter('search', value || undefined)
}

function onCategoryChange(value: string) {
  reputation.setFilter('category', value || undefined)
}
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-3xl font-bold tracking-tight">
        Source Reputation
      </h1>
      <p class="text-muted-foreground">
        Quality scores and performance metrics for content sources
      </p>
    </div>

    <div
      v-if="reputation.isLoading.value && reputation.sources.value.length === 0"
      class="flex items-center justify-center py-12"
    >
      <Loader2 class="h-8 w-8 animate-spin text-muted-foreground" />
    </div>

    <Card
      v-else-if="reputation.error.value"
      class="border-destructive"
    >
      <CardContent class="pt-6">
        <p class="text-destructive">
          {{ reputation.error.value?.message || 'Unable to load source reputation data.' }}
        </p>
      </CardContent>
    </Card>

    <Card v-else-if="reputation.sources.value.length === 0 && !reputation.hasActiveFilters.value">
      <CardContent class="flex flex-col items-center justify-center py-12">
        <Star class="h-12 w-12 text-muted-foreground mb-4" />
        <h3 class="text-lg font-medium mb-2">
          No reputation data
        </h3>
        <p class="text-muted-foreground">
          Source reputation will be calculated as content is classified.
        </p>
      </CardContent>
    </Card>

    <template v-else>
      <Card>
        <CardHeader class="pb-4">
          <CardTitle class="text-base">
            Filter Sources
          </CardTitle>
        </CardHeader>
        <CardContent>
          <ReputationFilterBar
            :filters="reputation.filters.value"
            :has-active-filters="reputation.hasActiveFilters.value"
            :active-filter-count="reputation.activeFilterCount.value"
            :categories="categoryOptions"
            @update:search="onSearchChange"
            @update:category="onCategoryChange"
            @clear-filters="reputation.clearFilters"
          />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Source Quality Scores</CardTitle>
          <CardDescription>Based on content quality and classification results</CardDescription>
        </CardHeader>
        <CardContent class="p-0">
          <ReputationTable
            :sources="reputation.sources.value"
            :total="reputation.total.value"
            :is-loading="reputation.isLoading.value"
            :page="reputation.page.value"
            :page-size="reputation.pageSize.value"
            :total-pages="reputation.totalPages.value"
            :allowed-page-sizes="reputation.allowedPageSizes"
            :sort-by="reputation.sortBy.value"
            :sort-order="reputation.sortOrder.value"
            :has-active-filters="reputation.hasActiveFilters.value"
            :on-sort="reputation.toggleSort"
            :on-page-change="reputation.setPage"
            :on-page-size-change="reputation.setPageSize"
            :on-clear-filters="reputation.clearFilters"
          />
        </CardContent>
      </Card>
    </template>
  </div>
</template>
