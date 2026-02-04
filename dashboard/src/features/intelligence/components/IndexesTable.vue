<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import {
  ChevronLeft,
  ChevronRight,
  MoreHorizontal,
  ArrowUp,
  ArrowDown,
  ArrowUpDown,
  Trash2,
  Eye,
  Database,
} from 'lucide-vue-next'
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

function getSortIcon(column: string) {
  if (indexes.sortBy.value !== column) return ArrowUpDown
  return indexes.sortOrder.value === 'asc' ? ArrowUp : ArrowDown
}

function handleSort(column: string) {
  indexes.toggleSort(column)
}

const pageNumbers = computed(() => {
  const current = indexes.page.value
  const total = indexes.totalPages.value
  const pages: (number | string)[] = []

  if (total <= 7) {
    for (let i = 1; i <= total; i++) pages.push(i)
  } else {
    pages.push(1)
    if (current > 3) pages.push('...')
    for (let i = Math.max(2, current - 1); i <= Math.min(total - 1, current + 1); i++) {
      pages.push(i)
    }
    if (current < total - 2) pages.push('...')
    pages.push(total)
  }

  return pages
})

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

function goToPage(page: number | string) {
  if (typeof page === 'number') {
    indexes.setPage(page)
  }
}

function handlePageSizeChange(event: Event) {
  const target = event.target as HTMLSelectElement
  indexes.setPageSize(Number(target.value))
}
</script>

<template>
  <div class="space-y-4">
    <!-- Table -->
    <div class="rounded-md border">
      <table class="w-full">
        <thead>
          <tr class="border-b bg-muted/50">
            <th
              v-for="col in sortableColumns"
              :key="col.key"
              class="px-4 py-3 text-left text-sm font-medium text-muted-foreground cursor-pointer hover:text-foreground transition-colors"
              @click="handleSort(col.key)"
            >
              <div class="flex items-center gap-1">
                {{ col.label }}
                <component
                  :is="getSortIcon(col.key)"
                  :class="[
                    'h-4 w-4',
                    indexes.sortBy.value === col.key ? 'text-foreground' : 'text-muted-foreground/50'
                  ]"
                />
              </div>
            </th>
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

    <!-- Pagination -->
    <div
      v-if="indexes.totalPages.value > 1 || indexes.totalIndexes.value > 0"
      class="flex items-center justify-between border-t pt-4"
    >
      <p class="text-sm text-muted-foreground">
        Showing {{ (indexes.page.value - 1) * indexes.pageSize.value + 1 }} to
        {{ Math.min(indexes.page.value * indexes.pageSize.value, indexes.totalIndexes.value) }}
        of {{ indexes.totalIndexes.value }} indexes
      </p>

      <div class="flex items-center gap-4">
        <!-- Page Size Selector -->
        <div class="flex items-center gap-2">
          <span class="text-sm text-muted-foreground">Show:</span>
          <select
            :value="indexes.pageSize.value"
            class="rounded-md border bg-background px-2 py-1 text-sm"
            @change="handlePageSizeChange"
          >
            <option
              v-for="size in indexes.allowedPageSizes"
              :key="size"
              :value="size"
            >
              {{ size }}
            </option>
          </select>
        </div>

        <!-- Page Numbers -->
        <div class="flex items-center gap-1">
          <Button
            variant="outline"
            size="sm"
            :disabled="indexes.page.value === 1"
            @click="goToPage(indexes.page.value - 1)"
          >
            <ChevronLeft class="h-4 w-4" />
          </Button>

          <template
            v-for="page in pageNumbers"
            :key="page"
          >
            <Button
              v-if="typeof page === 'number'"
              :variant="page === indexes.page.value ? 'default' : 'outline'"
              size="sm"
              class="min-w-9"
              @click="goToPage(page)"
            >
              {{ page }}
            </Button>
            <span
              v-else
              class="px-2 text-muted-foreground"
            >...</span>
          </template>

          <Button
            variant="outline"
            size="sm"
            :disabled="indexes.page.value === indexes.totalPages.value"
            @click="goToPage(indexes.page.value + 1)"
          >
            <ChevronRight class="h-4 w-4" />
          </Button>
        </div>
      </div>
    </div>
  </div>
</template>
