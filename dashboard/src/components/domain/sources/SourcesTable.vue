<script setup lang="ts">
import { useRouter } from 'vue-router'
import { formatDateShort } from '@/lib/utils'
import { Loader2, Pencil, Trash2 } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { DataTablePagination, SortableColumnHeader } from '@/components/common'
import { Skeleton } from '@/components/ui/skeleton'
import type { Source } from '@/features/scheduling/api/sources'

interface Props {
  sources: Source[]
  total: number
  isLoading: boolean
  page: number
  pageSize: number
  totalPages: number
  allowedPageSizes: readonly number[]
  sortBy: string
  sortOrder: 'asc' | 'desc'
  hasActiveFilters: boolean
  deletingId?: string | null
  onSort: (key: string) => void
  onPageChange: (page: number) => void
  onPageSizeChange: (size: number) => void
  onClearFilters: () => void
  onEdit: (id: string) => void
  onDelete: (id: string) => void
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'edit', id: string): void
  (e: 'delete', id: string): void
}>()

const router = useRouter()

const sortableColumns = [
  { key: 'name', label: 'Name' },
  { key: 'url', label: 'URL' },
  { key: 'enabled', label: 'Status' },
  { key: 'created_at', label: 'Created' },
] as const

function isEnabled(source: Source): boolean {
  return source.enabled ?? source.is_enabled ?? false
}

function editSource(id: string) {
  emit('edit', id)
  props.onEdit(id)
}

function deleteSource(id: string) {
  emit('delete', id)
  props.onDelete(id)
}
</script>

<template>
  <div class="space-y-4">
    <div class="rounded-md border">
      <table class="w-full">
        <thead>
          <tr class="border-b bg-muted/50">
            <SortableColumnHeader
              v-for="col in sortableColumns"
              :key="col.key"
              :label="col.label"
              :sort-key="col.key"
              :current-sort-by="sortBy"
              :current-sort-order="sortOrder"
              @sort="onSort(col.key)"
            />
            <th class="px-4 py-3 text-right text-sm font-medium text-muted-foreground">
              Actions
            </th>
          </tr>
        </thead>
        <tbody>
          <template v-if="isLoading">
            <tr
              v-for="i in 5"
              :key="i"
              class="border-b"
            >
              <td class="px-4 py-3">
                <Skeleton class="h-4 w-32" />
              </td>
              <td class="px-4 py-3">
                <Skeleton class="h-4 w-48" />
              </td>
              <td class="px-4 py-3">
                <Skeleton class="h-5 w-16" />
              </td>
              <td class="px-4 py-3">
                <Skeleton class="h-4 w-24" />
              </td>
              <td class="px-4 py-3 text-right">
                <Skeleton class="ml-auto h-8 w-8" />
              </td>
            </tr>
          </template>

          <tr
            v-else-if="sources.length === 0"
            class="border-b"
          >
            <td
              colspan="5"
              class="px-4 py-12 text-center"
            >
              <p class="text-sm text-muted-foreground">
                {{ hasActiveFilters ? 'No sources match your filters' : 'No sources found' }}
              </p>
              <Button
                v-if="hasActiveFilters"
                variant="outline"
                size="sm"
                class="mt-2"
                @click="onClearFilters"
              >
                Clear filters
              </Button>
            </td>
          </tr>

          <tr
            v-for="source in sources"
            v-else
            :key="source.id"
            class="border-b transition-colors hover:bg-muted/50 cursor-pointer"
            @click="router.push(`/scheduling/sources/${source.id}/edit`)"
          >
            <td class="px-4 py-3 text-sm font-medium">
              {{ source.name }}
            </td>
            <td class="px-4 py-3 text-sm">
              <a
                :href="source.url"
                target="_blank"
                class="text-primary hover:underline truncate block max-w-xs"
                @click.stop
              >
                {{ source.url }}
              </a>
            </td>
            <td class="px-4 py-3">
              <Badge :variant="isEnabled(source) ? 'success' : 'secondary'">
                {{ isEnabled(source) ? 'Active' : 'Inactive' }}
              </Badge>
            </td>
            <td class="px-4 py-3 text-sm text-muted-foreground">
              {{ formatDateShort(source.created_at) }}
            </td>
            <td
              class="px-4 py-3 text-right"
              @click.stop
            >
              <div class="flex justify-end gap-2">
                <Button
                  variant="ghost"
                  size="icon"
                  @click="editSource(source.id)"
                >
                  <Pencil class="h-4 w-4" />
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  :disabled="deletingId === source.id"
                  @click="deleteSource(source.id)"
                >
                  <Loader2
                    v-if="deletingId === source.id"
                    class="h-4 w-4 animate-spin"
                  />
                  <Trash2
                    v-else
                    class="h-4 w-4 text-destructive"
                  />
                </Button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <DataTablePagination
      :page="page"
      :page-size="pageSize"
      :total="total"
      :total-pages="totalPages"
      :allowed-page-sizes="allowedPageSizes"
      item-label="sources"
      @update:page="onPageChange"
      @update:page-size="onPageSizeChange"
    />
  </div>
</template>
