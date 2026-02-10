<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { Loader2, Globe, Plus, Upload } from 'lucide-vue-next'
import { sourcesApi } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { SourcesFilterBar, SourcesTable } from '@/components/domain/sources'
import { ImportExcelModal } from '@/components/common'
import { useSourcesTable } from '@/features/scheduling'

const router = useRouter()
const sourcesTable = useSourcesTable()
const deleting = ref<string | null>(null)
const importExcelModalRef = ref<InstanceType<typeof ImportExcelModal> | null>(null)

function editSource(id: string) {
  router.push(`/scheduling/sources/${id}/edit`)
}

async function deleteSource(id: string) {
  if (!confirm('Are you sure you want to delete this source?')) return
  try {
    deleting.value = id
    await sourcesApi.delete(id)
    await sourcesTable.refetch()
  } catch (err) {
    console.error('Error deleting source:', err)
  } finally {
    deleting.value = null
  }
}

function onSearchChange(value: string) {
  sourcesTable.setFilter('search', value || undefined)
}

function onEnabledChange(value: boolean | undefined) {
  sourcesTable.setFilter('enabled', value)
}

function openImportExcel() {
  importExcelModalRef.value?.open()
}

function onSourcesImported() {
  sourcesTable.refetch()
}
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-3xl font-bold tracking-tight">
          Sources
        </h1>
        <p class="text-muted-foreground">
          Manage content sources for crawling
        </p>
      </div>
      <div class="flex gap-2">
        <Button
          variant="outline"
          @click="openImportExcel"
        >
          <Upload class="mr-2 h-4 w-4" />
          Import Excel
        </Button>
        <Button @click="router.push('/scheduling/sources/new')">
          <Plus class="mr-2 h-4 w-4" />
          Add Source
        </Button>
      </div>
    </div>

    <div
      v-if="sourcesTable.isLoading.value && sourcesTable.sources.value.length === 0"
      class="flex items-center justify-center py-12"
    >
      <Loader2 class="h-8 w-8 animate-spin text-muted-foreground" />
    </div>

    <Card
      v-else-if="sourcesTable.error.value"
      class="border-destructive"
    >
      <CardContent class="pt-6">
        <p class="text-destructive">
          {{ sourcesTable.error.value?.message || 'Unable to load sources.' }}
        </p>
      </CardContent>
    </Card>

    <Card v-else-if="sourcesTable.sources.value.length === 0 && !sourcesTable.hasActiveFilters.value">
      <CardContent class="flex flex-col items-center justify-center py-12">
        <Globe class="h-12 w-12 text-muted-foreground mb-4" />
        <h3 class="text-lg font-medium mb-2">
          No sources configured
        </h3>
        <p class="text-muted-foreground mb-4">
          Add your first source to start crawling content.
        </p>
        <Button @click="router.push('/scheduling/sources/new')">
          <Plus class="mr-2 h-4 w-4" />
          Add Source
        </Button>
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
          <SourcesFilterBar
            :filters="sourcesTable.filters.value"
            :has-active-filters="sourcesTable.hasActiveFilters.value"
            :active-filter-count="sourcesTable.activeFilterCount.value"
            @update:search="onSearchChange"
            @update:enabled="onEnabledChange"
            @clear-filters="sourcesTable.clearFilters"
          />
        </CardContent>
      </Card>

      <Card>
        <CardContent class="p-0">
          <SourcesTable
            :sources="sourcesTable.sources.value"
            :total="sourcesTable.total.value"
            :is-loading="sourcesTable.isLoading.value"
            :page="sourcesTable.page.value"
            :page-size="sourcesTable.pageSize.value"
            :total-pages="sourcesTable.totalPages.value"
            :allowed-page-sizes="sourcesTable.allowedPageSizes"
            :sort-by="sourcesTable.sortBy.value"
            :sort-order="sourcesTable.sortOrder.value"
            :has-active-filters="sourcesTable.hasActiveFilters.value"
            :deleting-id="deleting"
            :on-sort="sourcesTable.toggleSort"
            :on-page-change="sourcesTable.setPage"
            :on-page-size-change="sourcesTable.setPageSize"
            :on-clear-filters="sourcesTable.clearFilters"
            :on-edit="editSource"
            :on-delete="deleteSource"
          />
        </CardContent>
      </Card>
    </template>

    <ImportExcelModal
      ref="importExcelModalRef"
      @imported="onSourcesImported"
    />
  </div>
</template>
