<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Loader2, Search, FileText } from 'lucide-vue-next'
import { indexManagerApi } from '@/api/client'
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
}

const route = useRoute()
const router = useRouter()

const indexName = computed(() => route.params.index_name as string)

const loading = ref(true)
const error = ref<string | null>(null)
const indexInfo = ref<Record<string, unknown> | null>(null)
const documents = ref<Document[]>([])
const searchQuery = ref('')
const loadingDocs = ref(false)

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
      pagination: { page: 1, size: 20 },
    })
    documents.value = response.data?.documents || []
  } catch (err) {
    console.error('Error loading documents:', err)
  } finally {
    loadingDocs.value = false
  }
}

const viewDocument = (docId: string) => {
  router.push(`/intelligence/indexes/${indexName.value}/documents/${docId}`)
}

const formatDate = (date: string) => date ? new Date(date).toLocaleDateString() : 'N/A'

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
                {{ ((indexInfo as Record<string, unknown>)?.docs_count as number || 0).toLocaleString() }}
              </dd>
            </div>
            <div>
              <dt class="text-sm text-muted-foreground">
                Size
              </dt>
              <dd class="text-2xl font-bold">
                {{ (indexInfo as Record<string, unknown>)?.size || 'N/A' }}
              </dd>
            </div>
            <div>
              <dt class="text-sm text-muted-foreground">
                Health
              </dt>
              <dd>
                <Badge variant="success">
                  {{ (indexInfo as Record<string, unknown>)?.health || 'unknown' }}
                </Badge>
              </dd>
            </div>
            <div>
              <dt class="text-sm text-muted-foreground">
                Type
              </dt>
              <dd class="text-lg font-medium">
                {{ (indexInfo as Record<string, unknown>)?.type || 'content' }}
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
                @keyup.enter="loadDocuments"
              />
              <Button
                variant="outline"
                @click="loadDocuments"
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
                  Date
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
                  {{ formatDate(doc.created_at || '') }}
                </td>
              </tr>
            </tbody>
          </table>
        </CardContent>
      </Card>
    </template>
  </div>
</template>
