<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Loader2, Search, FileText } from 'lucide-vue-next'
import { indexManagerApi } from '@/api/client'
import type { GetIndexResponse } from '@/types/indexManager'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'

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

const indexName = computed(() => route.params.index_name as string)

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

const previousPage = () => {
  if (currentPage.value > 1) {
    currentPage.value--
    loadDocuments()
  }
}

const nextPage = () => {
  if (currentPage.value < totalPages.value) {
    currentPage.value++
    loadDocuments()
  }
}

const goToPage = (page: number) => {
  currentPage.value = page
  loadDocuments()
}

const onPageSizeChange = () => {
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

const formatDate = (date: string) => {
  if (!date) return 'N/A'
  try {
    return new Date(date).toLocaleDateString()
  } catch {
    return 'N/A'
  }
}

const formatDateTime = (date: string) => {
  if (!date) return 'N/A'
  try {
    return new Date(date).toLocaleString()
  } catch {
    return 'N/A'
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
              </tr>
            </thead>
            <tbody class="divide-y">
              <tr 
                v-for="doc in documents" 
                :key="doc.id" 
                class="hover:bg-muted/50 cursor-pointer"
                @click="viewDocument(doc.id)"
              >
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
                  {{ formatDateTime(doc.created_at || doc.crawled_at || '') }}
                </td>
                <td class="px-6 py-4 text-sm text-muted-foreground">
                  {{ formatDate(doc.published_date || '') }}
                </td>
              </tr>
            </tbody>
          </table>
          
          <!-- Pagination -->
          <div
            v-if="totalPages > 1"
            class="px-6 py-4 border-t flex items-center justify-between"
          >
            <div class="flex-1 flex justify-between sm:hidden">
              <Button
                variant="outline"
                :disabled="currentPage === 1"
                @click="previousPage"
              >
                Previous
              </Button>
              <Button
                variant="outline"
                :disabled="currentPage >= totalPages"
                @click="nextPage"
              >
                Next
              </Button>
            </div>
            <div class="hidden sm:flex-1 sm:flex sm:items-center sm:justify-between">
              <div>
                <p class="text-sm text-muted-foreground">
                  Showing
                  <span class="font-medium">{{ ((currentPage - 1) * pageSize) + 1 }}</span>
                  to
                  <span class="font-medium">{{ Math.min(currentPage * pageSize, totalHits) }}</span>
                  of
                  <span class="font-medium">{{ totalHits }}</span>
                  results
                </p>
              </div>
              <div class="flex items-center gap-2">
                <select
                  v-model="pageSize"
                  class="px-3 py-1.5 border border-input rounded-md text-sm bg-background"
                  @change="onPageSizeChange"
                >
                  <option :value="10">
                    10 per page
                  </option>
                  <option :value="20">
                    20 per page
                  </option>
                  <option :value="50">
                    50 per page
                  </option>
                  <option :value="100">
                    100 per page
                  </option>
                </select>
                <nav
                  class="relative z-0 inline-flex rounded-md shadow-sm -space-x-px"
                  aria-label="Pagination"
                >
                  <Button
                    variant="outline"
                    size="sm"
                    :disabled="currentPage === 1"
                    class="rounded-r-none"
                    @click="goToPage(1)"
                  >
                    First
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    :disabled="currentPage === 1"
                    class="rounded-none border-l-0"
                    @click="previousPage"
                  >
                    Previous
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    :disabled="currentPage >= totalPages"
                    class="rounded-none border-l-0"
                    @click="nextPage"
                  >
                    Next
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    :disabled="currentPage >= totalPages"
                    class="rounded-l-none border-l-0"
                    @click="goToPage(totalPages)"
                  >
                    Last
                  </Button>
                </nav>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>
    </template>
  </div>
</template>
