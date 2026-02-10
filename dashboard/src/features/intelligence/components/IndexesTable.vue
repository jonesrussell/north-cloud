<script setup lang="ts">
import { useRouter } from 'vue-router'
import { MoreHorizontal, Trash2, Eye, Database } from 'lucide-vue-next'
import { DataTablePagination, SortableColumnHeader } from '@/components/common'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Skeleton } from '@/components/ui/skeleton'
import { useIndexes } from '../composables/useIndexes'
import type { Index, HealthStatus } from '@/types/indexManager'

interface Props {
  showActions?: boolean
  onRowClick?: (index: Index) => void
}

const props = withDefaults(defineProps<Props>(), {
  showActions: true,
  onRowClick: undefined,
})

const emit = defineEmits<{
  (e: 'view', index: Index): void
  (e: 'delete', index: Index): void
}>()

const router = useRouter()
const indexes = useIndexes()

type BadgeVariant = 'default' | 'secondary' | 'destructive' | 'outline' | 'success' | 'warning'

const healthVariants: Record<HealthStatus, BadgeVariant> = {
  green: 'success',
  yellow: 'warning',
  red: 'destructive',
}

// Define sortable columns
const sortableColumns = [
  { key: 'name', label: 'Name' },
  { key: 'type', label: 'Type' },
  { key: 'health', label: 'Health' },
  { key: 'document_count', label: 'Documents' },
  { key: 'size', label: 'Size' },
] as const

function handleSort(column: string) {
  indexes.toggleSort(column)
}

function formatNumber(num: number | undefined): string {
  if (num === undefined) return '0'
  return num.toLocaleString()
}

function formatType(type: string): string {
  return type.replace(/_/g, ' ').replace(/\b\w/g, (l) => l.toUpperCase())
}

function handleRowClick(index: Index) {
  if (props.onRowClick) {
    props.onRowClick(index)
  } else {
    router.push({ name: 'intelligence-index-detail', params: { index_name: index.name } })
  }
}

</script>

<template>
  <div class="space-y-4">
    <!-- Table -->
    <div class="rounded-md border">
      <table class="w-full">
        <thead>
          <tr class="border-b bg-muted/50">
            <SortableColumnHeader
              v-for="col in sortableColumns"
              :key="col.key"
              :label="col.label"
              :sort-key="col.key"
              :current-sort-by="indexes.sortBy.value"
              :current-sort-order="indexes.sortOrder.value"
              @sort="handleSort(col.key)"
            />
            <th
              v-if="showActions"
              class="px-4 py-3 text-right text-sm font-medium text-muted-foreground"
            >
              Actions
            </th>
          </tr>
        </thead>
        <tbody>
          <!-- Loading State -->
          <template v-if="indexes.isLoading.value">
            <tr
              v-for="i in 5"
              :key="i"
              class="border-b"
            >
              <td class="px-4 py-3">
                <Skeleton class="h-4 w-48" />
              </td>
              <td class="px-4 py-3">
                <Skeleton class="h-4 w-24" />
              </td>
              <td class="px-4 py-3">
                <Skeleton class="h-5 w-16" />
              </td>
              <td class="px-4 py-3">
                <Skeleton class="h-4 w-20" />
              </td>
              <td class="px-4 py-3">
                <Skeleton class="h-4 w-16" />
              </td>
              <td
                v-if="showActions"
                class="px-4 py-3"
              >
                <Skeleton class="ml-auto h-8 w-8" />
              </td>
            </tr>
          </template>

          <!-- Empty State -->
          <tr v-else-if="indexes.indexes.value.length === 0">
            <td
              :colspan="showActions ? 6 : 5"
              class="px-4 py-12 text-center"
            >
              <div class="flex flex-col items-center gap-2">
                <Database class="h-8 w-8 text-muted-foreground" />
                <p class="text-sm text-muted-foreground">
                  {{ indexes.hasActiveFilters.value ? 'No indexes match your filters' : 'No indexes found' }}
                </p>
                <Button
                  v-if="indexes.hasActiveFilters.value"
                  variant="outline"
                  size="sm"
                  @click="indexes.clearFilters()"
                >
                  Clear filters
                </Button>
              </div>
            </td>
          </tr>

          <!-- Data Rows -->
          <tr
            v-for="index in indexes.indexes.value"
            v-else
            :key="index.name"
            class="border-b transition-colors hover:bg-muted/50 cursor-pointer"
            @click="handleRowClick(index)"
          >
            <td class="px-4 py-3">
              <code class="rounded bg-muted px-1.5 py-0.5 text-sm font-mono">
                {{ index.name }}
              </code>
            </td>
            <td class="px-4 py-3 text-sm text-muted-foreground">
              {{ formatType(index.type) }}
            </td>
            <td class="px-4 py-3">
              <Badge
                v-if="index.health"
                :variant="healthVariants[index.health]"
              >
                {{ index.health }}
              </Badge>
              <span
                v-else
                class="text-sm text-muted-foreground"
              >—</span>
            </td>
            <td class="px-4 py-3 text-sm tabular-nums">
              {{ formatNumber(index.document_count) }}
            </td>
            <td class="px-4 py-3 text-sm text-muted-foreground">
              {{ index.size || '—' }}
            </td>
            <td
              v-if="showActions"
              class="px-4 py-3 text-right"
              @click.stop
            >
              <DropdownMenu>
                <DropdownMenuTrigger as-child>
                  <Button
                    variant="ghost"
                    size="sm"
                    class="h-8 w-8 p-0"
                  >
                    <MoreHorizontal class="h-4 w-4" />
                    <span class="sr-only">Open menu</span>
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem @click="emit('view', index)">
                    <Eye class="mr-2 h-4 w-4" />
                    View Details
                  </DropdownMenuItem>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem
                    class="text-destructive focus:text-destructive"
                    @click="emit('delete', index)"
                  >
                    <Trash2 class="mr-2 h-4 w-4" />
                    Delete
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <DataTablePagination
      :page="indexes.page.value"
      :page-size="indexes.pageSize.value"
      :total="indexes.totalIndexes.value"
      :total-pages="indexes.totalPages.value"
      :allowed-page-sizes="indexes.allowedPageSizes"
      item-label="indexes"
      @update:page="indexes.setPage"
      @update:page-size="indexes.setPageSize"
    />
  </div>
</template>
