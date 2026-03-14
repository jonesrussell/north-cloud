<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { CheckCircle, XCircle, RefreshCw, Filter, ExternalLink, BarChart2 } from 'lucide-vue-next'
import { verificationApi, type PendingItem, type EntityType } from '@/api/verification'
import { useToast } from '@/composables/useToast'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { DataTablePagination, BulkActionsToolbar, PageHeader, LoadingSpinner, ErrorAlert } from '@/components/common'
import { useBulkOperations } from '@/composables/useBulkOperations'
import { formatDate } from '@/lib/utils'

const router = useRouter()
const { toast } = useToast()
const bulkOps = useBulkOperations()

const loading = ref(true)
const error = ref<string | null>(null)
const items = ref<PendingItem[]>([])
const total = ref(0)

const typeFilter = ref<'' | 'person' | 'band_office'>('')
const page = ref(1)
const pageSize = ref(25)

const allowedPageSizes = [10, 25, 50] as const

const filteredItems = computed(() => {
  if (!typeFilter.value) return items.value
  return items.value.filter((item) => item.type === typeFilter.value)
})

const sortedItems = computed(() => {
  return [...filteredItems.value].sort((a, b) => {
    const aConf = (a.type === 'person' ? a.person?.verification_confidence : a.band_office?.verification_confidence) ?? 2
    const bConf = (b.type === 'person' ? b.person?.verification_confidence : b.band_office?.verification_confidence) ?? 2
    return aConf - bConf
  })
})

const totalPages = computed(() =>
  sortedItems.value.length === 0 ? 1 : Math.ceil(sortedItems.value.length / pageSize.value)
)

const paginatedItems = computed(() => {
  const start = (page.value - 1) * pageSize.value
  return sortedItems.value.slice(start, start + pageSize.value)
})

const selectedIds = computed(() => bulkOps.selectedIds.value)
const allPageSelected = computed(() =>
  paginatedItems.value.length > 0 && paginatedItems.value.every((item) => bulkOps.isSelected(itemId(item)))
)

function itemId(item: PendingItem): string {
  return item.type === 'person' ? (item.person?.id ?? '') : (item.band_office?.id ?? '')
}

function itemName(item: PendingItem): string {
  return item.type === 'person' ? (item.person?.name ?? '—') : (item.band_office?.city ?? '—')
}

function itemConfidence(item: PendingItem): number | undefined {
  return item.type === 'person' ? item.person?.verification_confidence : item.band_office?.verification_confidence
}

function itemIssues(item: PendingItem): string | undefined {
  return item.type === 'person' ? item.person?.verification_issues : item.band_office?.verification_issues
}

function itemSourceUrl(item: PendingItem): string | undefined {
  return item.type === 'person' ? item.person?.source_url : item.band_office?.source_url
}

function itemCreatedAt(item: PendingItem): string {
  return item.type === 'person' ? (item.person?.created_at ?? '') : (item.band_office?.created_at ?? '')
}

function confidenceClass(conf: number | undefined): string {
  if (conf === undefined) return 'text-gray-400'
  if (conf >= 0.9) return 'text-green-600 font-semibold'
  if (conf >= 0.5) return 'text-yellow-600 font-semibold'
  return 'text-red-600 font-semibold'
}

function confidenceLabel(conf: number | undefined): string {
  if (conf === undefined) return 'unscored'
  return (conf * 100).toFixed(0) + '%'
}

async function loadItems() {
  loading.value = true
  error.value = null
  try {
    const resp = await verificationApi.listPending({ limit: 200 })
    items.value = resp.items
    total.value = resp.total
    bulkOps.clearSelection()
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : 'Failed to load verification queue'
  } finally {
    loading.value = false
  }
}

function toggleAll() {
  if (allPageSelected.value) {
    paginatedItems.value.forEach((item) => bulkOps.deselect(itemId(item)))
  } else {
    paginatedItems.value.forEach((item) => bulkOps.select(itemId(item)))
  }
}

function goToDetail(item: PendingItem) {
  router.push({ name: 'operations-verification-detail', params: { type: item.type, id: itemId(item) } })
}

async function bulkVerify() {
  const ids = selectedIds.value
  if (!ids.length) return
  const type = inferBulkType(ids)
  if (!type) {
    toast({ title: 'Mixed types', description: 'Select only people or only band offices at once.', variant: 'destructive' })
    return
  }
  try {
    const result = await verificationApi.bulkVerify(ids, type)
    toast({ title: 'Verified', description: `${result.processed} records verified.` })
    await loadItems()
  } catch (e: unknown) {
    toast({ title: 'Error', description: e instanceof Error ? e.message : 'Bulk verify failed', variant: 'destructive' })
  }
}

async function bulkReject() {
  const ids = selectedIds.value
  if (!ids.length) return
  const type = inferBulkType(ids)
  if (!type) {
    toast({ title: 'Mixed types', description: 'Select only people or only band offices at once.', variant: 'destructive' })
    return
  }
  try {
    const result = await verificationApi.bulkReject(ids, type)
    toast({ title: 'Rejected', description: `${result.processed} records removed.` })
    await loadItems()
  } catch (e: unknown) {
    toast({ title: 'Error', description: e instanceof Error ? e.message : 'Bulk reject failed', variant: 'destructive' })
  }
}

function inferBulkType(ids: string[]): EntityType | null {
  const selectedItems = items.value.filter((item) => ids.includes(itemId(item)))
  const types = new Set(selectedItems.map((item) => item.type))
  if (types.size !== 1) return null
  return [...types][0] as EntityType
}

onMounted(loadItems)
</script>

<template>
  <div class="p-6 space-y-6">
    <PageHeader
      title="Verification Queue"
      description="Review AI-scored people and band offices awaiting approval"
    >
      <template #actions>
        <Button
          variant="outline"
          size="sm"
          @click="router.push({ name: 'operations-verification-stats' })"
        >
          <BarChart2 class="h-4 w-4 mr-1" />
          Stats
        </Button>
        <Button
          variant="outline"
          size="sm"
          @click="loadItems"
        >
          <RefreshCw class="h-4 w-4 mr-1" />
          Refresh
        </Button>
      </template>
    </PageHeader>

    <!-- Filters -->
    <div class="flex items-center gap-3">
      <Filter class="h-4 w-4 text-gray-500" />
      <select
        v-model="typeFilter"
        class="text-sm border rounded px-2 py-1"
        @change="page = 1"
      >
        <option value="">
          All types ({{ total }})
        </option>
        <option value="person">
          People
        </option>
        <option value="band_office">
          Band Offices
        </option>
      </select>
      <span class="text-sm text-gray-500">{{ sortedItems.length }} items</span>
    </div>

    <LoadingSpinner v-if="loading" />
    <ErrorAlert
      v-else-if="error"
      :message="error"
    />

    <template v-else>
      <!-- Bulk toolbar -->
      <BulkActionsToolbar
        v-if="selectedIds.length > 0"
        :count="selectedIds.length"
      >
        <Button
          size="sm"
          variant="outline"
          class="text-green-700 border-green-300"
          @click="bulkVerify"
        >
          <CheckCircle class="h-4 w-4 mr-1" />
          Verify
        </Button>
        <Button
          size="sm"
          variant="outline"
          class="text-red-700 border-red-300"
          @click="bulkReject"
        >
          <XCircle class="h-4 w-4 mr-1" />
          Reject
        </Button>
      </BulkActionsToolbar>

      <!-- Table -->
      <div class="rounded-md border overflow-auto">
        <table class="w-full text-sm">
          <thead class="bg-gray-50 border-b">
            <tr>
              <th class="p-3 w-8">
                <input
                  type="checkbox"
                  :checked="allPageSelected"
                  @change="toggleAll"
                >
              </th>
              <th class="p-3 text-left font-medium text-gray-600">
                Type
              </th>
              <th class="p-3 text-left font-medium text-gray-600">
                Name / Detail
              </th>
              <th class="p-3 text-left font-medium text-gray-600">
                Confidence ↑
              </th>
              <th class="p-3 text-left font-medium text-gray-600">
                Issues
              </th>
              <th class="p-3 text-left font-medium text-gray-600">
                Source
              </th>
              <th class="p-3 text-left font-medium text-gray-600">
                Added
              </th>
              <th class="p-3" />
            </tr>
          </thead>
          <tbody>
            <tr
              v-if="paginatedItems.length === 0"
              class="border-b"
            >
              <td
                colspan="8"
                class="p-8 text-center text-gray-500"
              >
                No pending items
              </td>
            </tr>
            <tr
              v-for="item in paginatedItems"
              :key="itemId(item)"
              class="border-b hover:bg-gray-50 cursor-pointer"
              @click.self="goToDetail(item)"
            >
              <td
                class="p-3"
                @click.stop
              >
                <input
                  type="checkbox"
                  :checked="bulkOps.isSelected(itemId(item))"
                  @change="bulkOps.toggle(itemId(item))"
                >
              </td>
              <td
                class="p-3"
                @click="goToDetail(item)"
              >
                <Badge :variant="item.type === 'person' ? 'default' : 'secondary'">
                  {{ item.type === 'person' ? 'Person' : 'Band Office' }}
                </Badge>
              </td>
              <td
                class="p-3 font-medium"
                @click="goToDetail(item)"
              >
                {{ itemName(item) }}
                <div
                  v-if="item.type === 'person' && item.person?.role"
                  class="text-xs text-gray-500 font-normal"
                >
                  {{ item.person.role }}
                </div>
              </td>
              <td
                class="p-3"
                :class="confidenceClass(itemConfidence(item))"
                @click="goToDetail(item)"
              >
                {{ confidenceLabel(itemConfidence(item)) }}
              </td>
              <td
                class="p-3 max-w-xs"
                @click="goToDetail(item)"
              >
                <span
                  v-if="itemIssues(item)"
                  class="text-xs text-gray-600 line-clamp-2"
                >{{ itemIssues(item) }}</span>
                <span
                  v-else
                  class="text-gray-400 text-xs"
                >—</span>
              </td>
              <td
                class="p-3"
                @click.stop
              >
                <a
                  v-if="itemSourceUrl(item)"
                  :href="itemSourceUrl(item)"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="text-blue-600 hover:underline inline-flex items-center gap-1"
                >
                  <ExternalLink class="h-3 w-3" />
                  View
                </a>
                <span
                  v-else
                  class="text-gray-400 text-xs"
                >—</span>
              </td>
              <td
                class="p-3 text-gray-500 text-xs"
                @click="goToDetail(item)"
              >
                {{ formatDate(itemCreatedAt(item)) }}
              </td>
              <td
                class="p-3"
                @click.stop
              >
                <Button
                  size="sm"
                  variant="ghost"
                  @click="goToDetail(item)"
                >
                  Review
                </Button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <DataTablePagination
        :page="page"
        :page-size="pageSize"
        :total="sortedItems.length"
        :total-pages="totalPages"
        :allowed-page-sizes="allowedPageSizes"
        @update:page="page = $event"
        @update:page-size="pageSize = $event; page = 1"
      />
    </template>
  </div>
</template>
