<script setup lang="ts">
import { formatDateShort } from '@/lib/utils'
import { Loader2, Pencil, Trash2, Check, X as XIcon } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { DataTablePagination, SortableColumnHeader } from '@/components/common'
import type { SocialAccount } from '@/types/socialPublisher'

interface Props {
  items: SocialAccount[]
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
  onEdit: (account: SocialAccount) => void
  onDelete: (id: string) => void
}

defineProps<Props>()

const sortableColumns = [
  { key: 'name', label: 'Name' },
  { key: 'platform', label: 'Platform' },
  { key: 'project', label: 'Project' },
  { key: 'created_at', label: 'Created' },
] as const

const platformColors: Record<string, string> = {
  x: 'bg-gray-900 text-white dark:bg-gray-100 dark:text-gray-900',
  facebook: 'bg-blue-600 text-white',
  instagram: 'bg-pink-600 text-white',
  linkedin: 'bg-blue-700 text-white',
  mastodon: 'bg-purple-600 text-white',
}

function getPlatformClass(platform: string): string {
  return platformColors[platform.toLowerCase()] ?? 'bg-muted text-muted-foreground'
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
            <th class="px-4 py-3 text-sm font-medium text-muted-foreground">
              Status
            </th>
            <th class="px-4 py-3 text-sm font-medium text-muted-foreground">
              Credentials
            </th>
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
              <td
                v-for="j in 7"
                :key="j"
                class="px-4 py-3"
              >
                <Skeleton class="h-4 w-20" />
              </td>
            </tr>
          </template>

          <tr
            v-else-if="items.length === 0"
            class="border-b"
          >
            <td
              colspan="7"
              class="px-4 py-12 text-center"
            >
              <p class="text-sm text-muted-foreground">
                {{ hasActiveFilters ? 'No accounts match your filters' : 'No accounts configured' }}
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
            v-for="account in items"
            v-else
            :key="account.id"
            class="border-b transition-colors hover:bg-muted/50"
          >
            <td class="px-4 py-3 text-sm font-medium">
              {{ account.name }}
            </td>
            <td class="px-4 py-3">
              <span
                :class="[
                  'inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium',
                  getPlatformClass(account.platform),
                ]"
              >
                {{ account.platform }}
              </span>
            </td>
            <td class="px-4 py-3 text-sm text-muted-foreground">
              {{ account.project || '\u2014' }}
            </td>
            <td class="px-4 py-3 text-sm text-muted-foreground">
              {{ formatDateShort(account.created_at) }}
            </td>
            <td class="px-4 py-3">
              <Badge :variant="account.enabled ? 'success' : 'secondary'">
                {{ account.enabled ? 'Active' : 'Inactive' }}
              </Badge>
            </td>
            <td class="px-4 py-3">
              <Check
                v-if="account.credentials_configured"
                class="h-4 w-4 text-green-600"
              />
              <XIcon
                v-else
                class="h-4 w-4 text-muted-foreground"
              />
            </td>
            <td class="px-4 py-3 text-right">
              <div class="flex justify-end gap-2">
                <Button
                  variant="ghost"
                  size="icon"
                  @click="onEdit(account)"
                >
                  <Pencil class="h-4 w-4" />
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  :disabled="deletingId === account.id"
                  @click="onDelete(account.id)"
                >
                  <Loader2
                    v-if="deletingId === account.id"
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
      item-label="accounts"
      @update:page="onPageChange"
      @update:page-size="onPageSizeChange"
    />
  </div>
</template>
