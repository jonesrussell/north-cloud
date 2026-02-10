<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { formatDate, formatDateShort } from '@/lib/utils'
import { ArrowLeft, Loader2, Search, FileText, RefreshCw } from 'lucide-vue-next'
import { indexManagerApi, classifierApi } from '@/api/client'
import type { GetIndexResponse } from '@/types/indexManager'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { DataTablePagination, BulkActionsToolbar } from '@/components/common'
import { useBulkOperations } from '@/composables/useBulkOperations'
import { useToast } from '@/composables/useToast'
import { reclassifyBulk } from '@/lib/reclassifyBulk'

interface Document {
  id: string
  title?: string
  url?: string
  quality_score?: number
  content_type?: string
  created_at?: string
  published_date?: string
  crawled_at?: string
}

const route = useRoute()
const router = useRouter()
const { toast } = useToast()

const indexName = computed(() => route.params.index_name as string)
const isClassifiedIndex = computed(() =>
  (indexName.value ?? '').endsWith('_classified_content')
)

const bulkOps = useBulkOperations()
const reclassifyingId = ref<string | null>(null)

const loading = ref(true)
const error = ref<string | null>(null)
const indexInfo = ref<GetIndexResponse | null>(null)
const documents = ref<Document[]>([])
const searchQuery = ref('')
const loadingDocs = ref(false)

// Pagination state
const currentPage = ref(1)
const pageSize = ref(20)
const totalHits = ref(0)
const totalPages = ref(0)
const allowedPageSizes = [10, 20, 50, 100] as const

const loadIndex = async () => {
  try {
    loading.value = true
    const response = await indexManagerApi.indexes.get(indexName.value)
    indexInfo.value = response.data
  } catch (err) {
    error.value = 'Unable to load index details.'
  } finally {
    loading.value = false
  }
}

const loadDocuments = async () => {
  try {
    loadingDocs.value = true
    const response = await indexManagerApi.documents.query(indexName.value, {
      query: searchQuery.value || undefined,
      pagination: { page: currentPage.value, size: pageSize.value },
    })
    documents.value = response.data?.documents || []
    totalHits.value = response.data?.total_hits || 0
    totalPages.value = response.data?.total_pages || 0
  } catch (err) {
    console.error('Error loading documents:', err)
  } finally {
    loadingDocs.value = false
  }
}

function onPageChange(page: number) {
  currentPage.value = page
  loadDocuments()
}

function onPageSizeChange(size: number) {
  pageSize.value = size
  currentPage.value = 1
  loadDocuments()
}

// Reset to page 1 when search query changes
const handleSearch = () => {
  currentPage.value = 1
  loadDocuments()
}

const viewDocument = (docId: string) => {
  router.push(`/intelligence/indexes/${indexName.value}/documents/${docId}`)
}

function confirmBulkReclassify(n: number): boolean {
  if (n >= 500) {
    return confirm(`Are you sure? This will reclassify ${n} documents.`)
  }
  if (n >= 100) {
    return confirm(`Reclassify ${n} documents?`)
  }
  return true
}

async function handlePerRowReclassify(doc: Document, event: Event) {
  event.stopPropagation()
  reclassifyingId.value = doc.id
  try {
    await classifierApi.classify.reclassify(doc.id)
    await loadDocuments()
    toast.success('Document reclassified')
  } catch (err: unknown) {
    const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error ?? 'Reclassification failed.'
    toast.error(msg)
  } finally {
    reclassifyingId.value = null
  }
}

async function handleBulkReclassify(selectedIds: string[]) {
  const n = selectedIds.length
  if (!confirmBulkReclassify(n)) return

  const result = await reclassifyBulk(selectedIds)
  bulkOps.clearSelection()
  await loadDocuments()

  if (result.succeeded > 0) {
    toast.success(`Reclassified ${result.succeeded} document${result.succeeded !== 1 ? 's' : ''}`)
  }
  if (result.failed > 0) {
    const sample = result.errors.slice(0, 5).map((e) => `${e.id}: ${e.error}`).join('; ')
    const suffix = result.errors.length > 5 ? '…' : ''
    toast.error(`${result.failed} failed: ${sample}${suffix}`)
  }
}

const getHealthVariant = (health: string | undefined) => {
  if (!health) return 'pending'
  switch (health.toLowerCase()) {
    case 'green':
      return 'success'
    case 'yellow':
      return 'warning'
    case 'red':
      return 'destructive'
    default:
      return 'pending'
  }
}

onMounted(() => {
  loadIndex()
  loadDocuments()
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center gap-4">
      <Button
        variant="ghost"
        size="icon"
        @click="router.push('/intelligence/indexes')"
      >
        <ArrowLeft class="h-5 w-5" />
      </Button>
      <div>
        <h1 class="text-3xl font-bold tracking-tight">
          {{ indexName }}
        </h1>
        <p class="text-muted-foreground">
          Index details and documents
        </p>
      </div>
    </div>

    <div
      v-if="loading"
      class="flex items-center justify-center py-12"
    >
      <Loader2 class="h-8 w-8 animate-spin text-muted-foreground" />
    </div>

    <Card
      v-else-if="error"
      class="border-destructive"
    >
      <CardContent class="pt-6">
        <p class="text-destructive">
          {{ error }}
        </p>
      </CardContent>
    </Card>

    <template v-else>
      <!-- Index Info -->
      <Card>
        <CardHeader>
          <CardTitle>Index Information</CardTitle>
        </CardHeader>
        <CardContent>
          <dl class="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div>
              <dt class="text-sm text-muted-foreground">
                Documents
              </dt>
              <dd class="text-2xl font-bold">
                {{ totalHits.toLocaleString() }}
              </dd>
            </div>
            <div>
              <dt class="text-sm text-muted-foreground">
                Size
              </dt>
              <dd class="text-2xl font-bold">
                {{ indexInfo?.size || 'N/A' }}
              </dd>
            </div>
            <div>
              <dt class="text-sm text-muted-foreground">
                Health
              </dt>
              <dd>
                <Badge :variant="getHealthVariant(indexInfo?.health)">
                  {{ indexInfo?.health || 'unknown' }}
                </Badge>
              </dd>
            </div>
            <div>
              <dt class="text-sm text-muted-foreground">
                Type
              </dt>
              <dd class="text-lg font-medium">
                {{ indexInfo?.type || 'content' }}
              </dd>
            </div>
          </dl>
        </CardContent>
      </Card>

      <!-- Documents -->
      <Card>
        <CardHeader>
          <div class="flex items-center justify-between">
            <div>
              <CardTitle>Documents</CardTitle>
              <CardDescription>Browse and search indexed content</CardDescription>
            </div>
            <div class="flex gap-2">
              <Input 
                v-model="searchQuery" 
                placeholder="Search documents..." 
                class="w-64"
                @keyup.enter="handleSearch"
              />
              <Button
                variant="outline"
                @click="handleSearch"
              >
                <Search class="h-4 w-4" />
              </Button>
            </div>
          </div>
        </CardHeader>
        <CardContent class="p-0">
          <div
            v-if="loadingDocs"
            class="flex justify-center py-8"
          >
            <Loader2 class="h-6 w-6 animate-spin" />
          </div>
          <div
            v-else-if="documents.length === 0"
            class="py-8 text-center text-muted-foreground"
          >
            <FileText class="h-12 w-12 mx-auto mb-4 text-muted-foreground/50" />
            <p>No documents found</p>
          </div>
          <table
            v-else
            class="w-full"
          >
            <thead class="border-b bg-muted/50">
              <tr>
                <th
                  v-if="isClassifiedIndex"
                  class="w-12 px-4 py-3"
                >
                  <input
                    type="checkbox"
                    :checked="bulkOps.selectAll && documents.length > 0"
                    class="rounded border-input"
                    @change="bulkOps.toggleSelectAll(documents)"
                  >
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                  Title
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                  Type
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                  Quality
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                  Created
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                  Published
                </th>
                <th
                  v-if="isClassifiedIndex"
                  class="px-6 py-3 text-right text-xs font-medium text-muted-foreground uppercase"
                >
                  Actions
                </th>
              </tr>
            </thead>
            <tbody class="divide-y">
              <tr
                v-for="doc in documents"
                :key="doc.id"
                class="hover:bg-muted/50 cursor-pointer"
                @click="viewDocument(doc.id)"
              >
                <td
                  v-if="isClassifiedIndex"
                  class="w-12 px-4 py-4"
                  @click.stop
                >
                  <input
                    type="checkbox"
                    :checked="bulkOps.isSelected(doc.id)"
                    class="rounded border-input"
                    @change="bulkOps.toggleItem(doc.id)"
                  >
                </td>
                <td class="px-6 py-4 text-sm">
                  <button class="text-primary hover:underline text-left truncate max-w-md block">
                    {{ doc.title || 'Untitled' }}
                  </button>
                </td>
                <td class="px-6 py-4 text-sm text-muted-foreground">
                  {{ doc.content_type || 'article' }}
                </td>
                <td class="px-6 py-4">
                  <Badge
                    v-if="doc.quality_score"
                    variant="secondary"
                  >
                    {{ doc.quality_score }}/100
                  </Badge>
                  <span
                    v-else
                    class="text-muted-foreground"
                  >—</span>
                </td>
                <td class="px-6 py-4 text-sm text-muted-foreground">
                  {{ formatDate(doc.created_at || doc.crawled_at || '') }}
                </td>
                <td class="px-6 py-4 text-sm text-muted-foreground">
                  {{ formatDateShort(doc.published_date || '') }}
                </td>
                <td
                  v-if="isClassifiedIndex"
                  class="px-6 py-4 text-right"
                  @click.stop
                >
                  <Button
                    variant="ghost"
                    size="sm"
                    :disabled="reclassifyingId === doc.id"
                    :title="'Reclassify'"
                    @click="handlePerRowReclassify(doc, $event)"
                  >
                    <Loader2
                      v-if="reclassifyingId === doc.id"
                      class="h-4 w-4 animate-spin"
                    />
                    <RefreshCw
                      v-else
                      class="h-4 w-4"
                    />
                  </Button>
                </td>
              </tr>
            </tbody>
          </table>
          
          <div class="px-6 pb-4">
            <DataTablePagination
              :page="currentPage"
              :page-size="pageSize"
              :total="totalHits"
              :total-pages="totalPages"
              :allowed-page-sizes="allowedPageSizes"
              item-label="documents"
              @update:page="onPageChange"
              @update:page-size="onPageSizeChange"
            />
          </div>
        </CardContent>
      </Card>

      <BulkActionsToolbar
        v-if="isClassifiedIndex"
        :selected-count="bulkOps.selectedCount"
        :selected-ids="bulkOps.selectedIds"
        :available-actions="[
          {
            id: 'reclassify',
            label: 'Reclassify selected',
            variant: 'primary',
            icon: RefreshCw,
            handler: handleBulkReclassify,
          },
        ]"
        @cancel="bulkOps.clearSelection()"
      />
    </template>
  </div>
</template>
