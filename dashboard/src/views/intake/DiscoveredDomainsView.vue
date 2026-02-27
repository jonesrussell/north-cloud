<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useQueryClient } from '@tanstack/vue-query'
import {
  Loader2,
  Globe,
  RefreshCw,
  MoreHorizontal,
  Eye,
  ClipboardCheck,
  EyeOff,
  ArrowUpCircle,
} from 'lucide-vue-next'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
} from '@/components/ui/dropdown-menu'
import { DataTablePagination, SortableColumnHeader } from '@/components/common'
import { useDiscoveredDomainsTable } from '@/features/intake'
import { crawlerApi } from '@/api/client'
import { formatRelativeTime } from '@/lib/utils'
import type { DiscoveredDomain } from '@/features/intake/api/discoveredDomains'

const router = useRouter()
const queryClient = useQueryClient()
const table = useDiscoveredDomainsTable()

const selectedDomains = ref<Set<string>>(new Set())
const bulkUpdating = ref(false)

// --- Filters ---

const searchInput = ref('')
const statusFilter = ref('')
const minScoreInput = ref('')
const hideExistingChecked = ref(false)

function onSearchChange() {
  table.setFilter('search', searchInput.value || undefined)
}

function onStatusChange(event: Event) {
  const target = event.target as HTMLSelectElement
  statusFilter.value = target.value
  table.setFilter('status', target.value || undefined)
}

function onMinScoreChange() {
  const parsed = minScoreInput.value ? Number(minScoreInput.value) : undefined
  table.setFilter('min_score', parsed)
}

function onHideExistingChange() {
  table.setFilter('hide_existing', hideExistingChecked.value || undefined)
}

function clearAllFilters() {
  searchInput.value = ''
  statusFilter.value = ''
  minScoreInput.value = ''
  hideExistingChecked.value = false
  table.clearFilters()
}

// --- Selection ---

const allOnPageSelected = computed(() => {
  const domains = table.domains.value
  if (domains.length === 0) return false
  return domains.every((d) => selectedDomains.value.has(d.domain))
})

function toggleSelectAll() {
  if (allOnPageSelected.value) {
    for (const d of table.domains.value) {
      selectedDomains.value.delete(d.domain)
    }
  } else {
    for (const d of table.domains.value) {
      selectedDomains.value.add(d.domain)
    }
  }
}

function toggleSelect(domain: string) {
  if (selectedDomains.value.has(domain)) {
    selectedDomains.value.delete(domain)
  } else {
    selectedDomains.value.add(domain)
  }
}

// --- Status helpers ---

type StatusVariant = 'info' | 'warning' | 'pending' | 'success' | 'outline'

function getStatusVariant(status: string): StatusVariant {
  switch (status) {
    case 'active': return 'info'
    case 'reviewing': return 'warning'
    case 'ignored': return 'pending'
    case 'promoted': return 'success'
    default: return 'outline'
  }
}

type ScoreVariant = 'success' | 'warning' | 'destructive'

const HIGH_SCORE_THRESHOLD = 70
const MID_SCORE_THRESHOLD = 40

function getScoreVariant(score: number): ScoreVariant {
  if (score >= HIGH_SCORE_THRESHOLD) return 'success'
  if (score >= MID_SCORE_THRESHOLD) return 'warning'
  return 'destructive'
}

function formatPercent(value: number | null): string {
  if (value === null || value === undefined) return '\u2014'
  return `${Math.round(value * 100)}%`
}

// --- Actions ---

async function updateDomainStatus(domain: string, status: string) {
  try {
    await crawlerApi.discoveredDomains.updateState(domain, { status })
    queryClient.invalidateQueries({ queryKey: ['discovered-domains'] })
  } catch (err: unknown) {
    console.error('Error updating domain status:', err)
  }
}

async function bulkUpdateStatus(status: string) {
  if (selectedDomains.value.size === 0) return
  try {
    bulkUpdating.value = true
    await crawlerApi.discoveredDomains.bulkUpdateState({
      domains: Array.from(selectedDomains.value),
      status,
    })
    selectedDomains.value.clear()
    queryClient.invalidateQueries({ queryKey: ['discovered-domains'] })
  } catch (err: unknown) {
    console.error('Error bulk updating domains:', err)
  } finally {
    bulkUpdating.value = false
  }
}

function navigateToDetail(domain: DiscoveredDomain) {
  router.push(`/intake/discovered-links/${encodeURIComponent(domain.domain)}`)
}

// --- Sortable columns ---

const sortableColumns = [
  { key: 'domain', label: 'Domain' },
  { key: 'link_count', label: 'Links' },
  { key: 'source_count', label: 'Sources' },
  { key: 'last_seen', label: 'Last Seen' },
] as const
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-3xl font-bold tracking-tight">
          Discovered Domains
        </h1>
        <p class="text-muted-foreground">
          Domains discovered during crawling, aggregated by host
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

    <!-- Loading -->
    <div
      v-if="table.isLoading.value && table.domains.value.length === 0"
      class="flex items-center justify-center py-12"
    >
      <Loader2 class="h-8 w-8 animate-spin text-muted-foreground" />
    </div>

    <!-- Error -->
    <Card
      v-else-if="table.error.value"
      class="border-destructive"
    >
      <CardContent class="pt-6">
        <p class="text-destructive">
          {{ (table.error.value as Error)?.message || 'Unable to load discovered domains.' }}
        </p>
      </CardContent>
    </Card>

    <!-- Empty (no filters) -->
    <Card
      v-else-if="table.domains.value.length === 0 && !table.hasActiveFilters.value"
      class="border-dashed"
    >
      <CardContent class="flex flex-col items-center justify-center py-12">
        <Globe class="h-12 w-12 text-muted-foreground mb-4" />
        <h3 class="text-lg font-medium mb-2">
          No discovered domains
        </h3>
        <p class="text-muted-foreground">
          Domains discovered during crawling will appear here.
        </p>
      </CardContent>
    </Card>

    <!-- Data -->
    <template v-else>
      <!-- Filters -->
      <Card>
        <CardHeader class="pb-4">
          <CardTitle class="text-base">
            Filter Domains
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div class="flex flex-wrap items-end gap-4">
            <div class="flex-1 min-w-48">
              <label class="block text-sm font-medium mb-1">Search</label>
              <Input
                v-model="searchInput"
                placeholder="Search domains..."
                @update:model-value="onSearchChange"
              />
            </div>

            <div class="min-w-36">
              <label class="block text-sm font-medium mb-1">Status</label>
              <select
                :value="statusFilter"
                class="flex h-10 w-full rounded-sm border border-input bg-background px-3 py-2 text-sm"
                @change="onStatusChange"
              >
                <option value="">
                  All
                </option>
                <option value="active">
                  Active
                </option>
                <option value="reviewing">
                  Reviewing
                </option>
                <option value="ignored">
                  Ignored
                </option>
                <option value="promoted">
                  Promoted
                </option>
              </select>
            </div>

            <div class="min-w-28">
              <label class="block text-sm font-medium mb-1">Min Score</label>
              <Input
                v-model="minScoreInput"
                type="number"
                placeholder="0"
                @update:model-value="onMinScoreChange"
              />
            </div>

            <div class="flex items-center gap-2 pb-2">
              <input
                id="hide-existing"
                v-model="hideExistingChecked"
                type="checkbox"
                class="h-4 w-4"
                @change="onHideExistingChange"
              >
              <label
                for="hide-existing"
                class="text-sm whitespace-nowrap"
              >Hide existing sources</label>
            </div>

            <Button
              v-if="table.hasActiveFilters.value"
              variant="ghost"
              size="sm"
              class="mb-0.5"
              @click="clearAllFilters"
            >
              Clear filters
              <Badge
                variant="secondary"
                class="ml-1"
              >
                {{ table.activeFilterCount.value }}
              </Badge>
            </Button>
          </div>
        </CardContent>
      </Card>

      <!-- Bulk Actions Bar -->
      <div
        v-if="selectedDomains.size > 0"
        class="flex items-center gap-3 rounded-md border bg-muted/50 px-4 py-3"
      >
        <span class="text-sm font-medium">
          {{ selectedDomains.size }} selected
        </span>
        <Button
          variant="outline"
          size="sm"
          :disabled="bulkUpdating"
          @click="bulkUpdateStatus('reviewing')"
        >
          <ClipboardCheck class="mr-1.5 h-3.5 w-3.5" />
          Mark Reviewing
        </Button>
        <Button
          variant="outline"
          size="sm"
          :disabled="bulkUpdating"
          @click="bulkUpdateStatus('ignored')"
        >
          <EyeOff class="mr-1.5 h-3.5 w-3.5" />
          Ignore
        </Button>
        <Button
          variant="ghost"
          size="sm"
          @click="selectedDomains.clear()"
        >
          Clear
        </Button>
      </div>

      <!-- Results Table -->
      <Card>
        <CardHeader>
          <CardTitle>Domains</CardTitle>
        </CardHeader>
        <CardContent class="p-0">
          <div class="space-y-4">
            <div class="rounded-md border">
              <table class="w-full">
                <thead>
                  <tr class="border-b bg-muted/50">
                    <!-- Checkbox column -->
                    <th class="w-12 px-4 py-3">
                      <input
                        type="checkbox"
                        class="h-4 w-4"
                        :checked="allOnPageSelected"
                        @change="toggleSelectAll"
                      >
                    </th>
                    <SortableColumnHeader
                      v-for="col in sortableColumns"
                      :key="col.key"
                      :label="col.label"
                      :sort-key="col.key"
                      :current-sort-by="table.sortBy.value"
                      :current-sort-order="table.sortOrder.value"
                      @sort="table.toggleSort(col.key)"
                    />
                    <th class="px-4 py-3 text-left text-sm font-medium text-muted-foreground">
                      OK %
                    </th>
                    <th class="px-4 py-3 text-left text-sm font-medium text-muted-foreground">
                      HTML %
                    </th>
                    <th class="px-4 py-3 text-left text-sm font-medium text-muted-foreground">
                      Status
                    </th>
                    <th class="px-4 py-3 text-right text-sm font-medium text-muted-foreground">
                      Actions
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <!-- Loading skeletons -->
                  <template v-if="table.isLoading.value">
                    <tr
                      v-for="i in 5"
                      :key="i"
                      class="border-b"
                    >
                      <td
                        v-for="j in 10"
                        :key="j"
                        class="px-4 py-3"
                      >
                        <Skeleton class="h-4 w-20" />
                      </td>
                    </tr>
                  </template>

                  <!-- Empty state with filters -->
                  <tr
                    v-else-if="table.domains.value.length === 0"
                    class="border-b"
                  >
                    <td
                      colspan="10"
                      class="px-4 py-12 text-center"
                    >
                      <p class="text-sm text-muted-foreground">
                        No domains match your filters
                      </p>
                      <Button
                        variant="outline"
                        size="sm"
                        class="mt-2"
                        @click="clearAllFilters"
                      >
                        Clear filters
                      </Button>
                    </td>
                  </tr>

                  <!-- Data rows -->
                  <tr
                    v-for="domain in table.domains.value"
                    v-else
                    :key="domain.domain"
                    class="border-b transition-colors hover:bg-muted/50 cursor-pointer"
                    @click="navigateToDetail(domain)"
                  >
                    <!-- Checkbox -->
                    <td
                      class="w-12 px-4 py-3"
                      @click.stop
                    >
                      <input
                        type="checkbox"
                        class="h-4 w-4"
                        :checked="selectedDomains.has(domain.domain)"
                        @change="toggleSelect(domain.domain)"
                      >
                    </td>

                    <!-- Domain -->
                    <td class="px-4 py-3">
                      <div class="flex items-center gap-2">
                        <span class="text-sm font-medium">
                          {{ domain.domain }}
                        </span>
                        <Badge
                          v-if="domain.is_existing_source"
                          variant="outline"
                          class="text-xs"
                        >
                          existing
                        </Badge>
                      </div>
                    </td>

                    <!-- Score -->
                    <td class="px-4 py-3">
                      <Badge :variant="getScoreVariant(domain.quality_score)">
                        {{ domain.quality_score }}
                      </Badge>
                    </td>

                    <!-- Links -->
                    <td class="px-4 py-3 text-sm text-muted-foreground">
                      {{ domain.link_count }}
                    </td>

                    <!-- Sources -->
                    <td class="px-4 py-3 text-sm text-muted-foreground">
                      {{ domain.source_count }}
                    </td>

                    <!-- Last Seen -->
                    <td class="px-4 py-3 text-sm text-muted-foreground">
                      {{ formatRelativeTime(domain.last_seen) }}
                    </td>

                    <!-- OK % -->
                    <td class="px-4 py-3 text-sm text-muted-foreground">
                      {{ formatPercent(domain.ok_ratio) }}
                    </td>

                    <!-- HTML % -->
                    <td class="px-4 py-3 text-sm text-muted-foreground">
                      {{ formatPercent(domain.html_ratio) }}
                    </td>

                    <!-- Status -->
                    <td class="px-4 py-3">
                      <Badge :variant="getStatusVariant(domain.status)">
                        {{ domain.status }}
                      </Badge>
                    </td>

                    <!-- Actions -->
                    <td
                      class="px-4 py-3 text-right"
                      @click.stop
                    >
                      <DropdownMenu>
                        <DropdownMenuTrigger>
                          <Button
                            variant="ghost"
                            size="xs"
                          >
                            <MoreHorizontal class="h-4 w-4" />
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end">
                          <DropdownMenuItem @select="navigateToDetail(domain)">
                            <Eye class="mr-2 h-4 w-4" />
                            View Detail
                          </DropdownMenuItem>
                          <DropdownMenuSeparator />
                          <DropdownMenuItem @select="updateDomainStatus(domain.domain, 'reviewing')">
                            <ClipboardCheck class="mr-2 h-4 w-4" />
                            Mark Reviewing
                          </DropdownMenuItem>
                          <DropdownMenuItem @select="updateDomainStatus(domain.domain, 'ignored')">
                            <EyeOff class="mr-2 h-4 w-4" />
                            Ignore
                          </DropdownMenuItem>
                          <DropdownMenuItem @select="updateDomainStatus(domain.domain, 'promoted')">
                            <ArrowUpCircle class="mr-2 h-4 w-4" />
                            Promote
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>

            <DataTablePagination
              :page="table.page.value"
              :page-size="table.pageSize.value"
              :total="table.total.value"
              :total-pages="table.totalPages.value"
              :allowed-page-sizes="table.allowedPageSizes"
              item-label="domains"
              @update:page="table.setPage"
              @update:page-size="table.setPageSize"
            />
          </div>
        </CardContent>
      </Card>
    </template>
  </div>
</template>
