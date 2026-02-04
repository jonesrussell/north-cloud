<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import {
  Loader2,
  AlertTriangle,
  ExternalLink,
  RefreshCw,
  CheckCircle,
  Eye,
  FileText,
} from 'lucide-vue-next'
import { indexManagerApi } from '@/api/client'
import type { Document, Index } from '@/types/indexManager'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'

const router = useRouter()

const loading = ref(true)
const error = ref<string | null>(null)
const documents = ref<Document[]>([])
const classifiedIndexes = ref<Index[]>([])

const totalHits = ref(0)

const loadClassifiedIndexes = async () => {
  try {
    const response = await indexManagerApi.indexes.list({ type: 'classified_content' })
    classifiedIndexes.value = response.data?.indices || []
  } catch (err) {
    console.error('Failed to load indexes:', err)
  }
}

const loadReviewQueue = async () => {
  try {
    loading.value = true
    error.value = null
    documents.value = []
    totalHits.value = 0

    // Query each classified_content index for review_required documents
    const allDocs: Document[] = []
    for (const index of classifiedIndexes.value) {
      try {
        const response = await indexManagerApi.documents.query(index.name, {
          filters: { review_required: true },
          pagination: { page: 1, size: 100 },
          sort: { field: 'crawled_at', order: 'desc' },
        })
        const docs = response.data?.documents || []
        // Add index name to each doc for navigation
        docs.forEach((d) => {
          if (!d.source_name) {
            d.source_name = index.source_name || index.name.replace('_classified_content', '')
          }
        })
        allDocs.push(...docs)
      } catch (indexErr) {
        console.error(`Failed to query ${index.name}:`, indexErr)
      }
    }

    // Sort all docs by crawled_at desc
    allDocs.sort((a, b) => {
      const dateA = a.crawled_at ? new Date(a.crawled_at).getTime() : 0
      const dateB = b.crawled_at ? new Date(b.crawled_at).getTime() : 0
      return dateB - dateA
    })

    documents.value = allDocs
    totalHits.value = allDocs.length
  } catch (err) {
    error.value = 'Unable to load review queue.'
  } finally {
    loading.value = false
  }
}

const refresh = async () => {
  await loadClassifiedIndexes()
  await loadReviewQueue()
}

const viewDocument = (doc: Document) => {
  const indexName = `${doc.source_name}_classified_content`
  router.push(`/intelligence/indexes/${indexName}/documents/${doc.id}`)
}

const formatDate = (date?: string) => {
  if (!date) return 'N/A'
  try {
    return new Date(date).toLocaleString()
  } catch {
    return 'N/A'
  }
}

const formatRelevance = (relevance?: string) => {
  if (!relevance) return 'unknown'
  return relevance.replace(/_/g, ' ')
}

const getRelevanceVariant = (relevance?: string) => {
  switch (relevance) {
    case 'direct':
      return 'destructive'
    case 'related':
      return 'warning'
    case 'peripheral':
      return 'secondary'
    default:
      return 'outline'
  }
}

onMounted(async () => {
  await loadClassifiedIndexes()
  await loadReviewQueue()
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-3xl font-bold tracking-tight">
          Review Queue
        </h1>
        <p class="text-muted-foreground">
          Articles flagged for manual review before publishing
        </p>
      </div>
      <Button
        variant="outline"
        :disabled="loading"
        @click="refresh"
      >
        <RefreshCw
          class="mr-2 h-4 w-4"
          :class="{ 'animate-spin': loading }"
        />
        Refresh
      </Button>
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

    <Card v-else-if="documents.length === 0">
      <CardContent class="flex flex-col items-center justify-center py-12">
        <CheckCircle class="h-12 w-12 text-green-500 mb-4" />
        <h3 class="text-lg font-medium mb-2">
          Queue is empty
        </h3>
        <p class="text-muted-foreground">
          No articles currently require manual review.
        </p>
      </CardContent>
    </Card>

    <Card v-else>
      <CardHeader>
        <CardTitle class="flex items-center gap-2">
          <AlertTriangle class="h-5 w-5 text-yellow-500" />
          Pending Review
        </CardTitle>
        <CardDescription>
          {{ totalHits }} article{{ totalHits !== 1 ? 's' : '' }} require manual review
        </CardDescription>
      </CardHeader>
      <CardContent class="p-0">
        <table class="w-full">
          <thead class="border-b bg-muted/50">
            <tr>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Article
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Source
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Crime Type
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Relevance
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Quality
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Crawled
              </th>
              <th class="px-6 py-3 text-right text-xs font-medium text-muted-foreground uppercase">
                Actions
              </th>
            </tr>
          </thead>
          <tbody class="divide-y">
            <tr
              v-for="doc in documents"
              :key="doc.id"
              class="hover:bg-muted/50"
            >
              <td class="px-6 py-4">
                <div class="max-w-md">
                  <p class="text-sm font-medium truncate">
                    {{ doc.title || 'Untitled' }}
                  </p>
                  <p class="text-xs text-muted-foreground truncate">
                    {{ doc.url }}
                  </p>
                </div>
              </td>
              <td class="px-6 py-4">
                <Badge variant="outline">
                  {{ doc.source_name }}
                </Badge>
              </td>
              <td class="px-6 py-4">
                <Badge
                  v-if="doc.crime?.primary_crime_type"
                  variant="secondary"
                >
                  {{ doc.crime.primary_crime_type }}
                </Badge>
                <span
                  v-else
                  class="text-sm text-muted-foreground"
                >-</span>
              </td>
              <td class="px-6 py-4">
                <Badge :variant="getRelevanceVariant(doc.crime?.relevance)">
                  {{ formatRelevance(doc.crime?.relevance) }}
                </Badge>
              </td>
              <td class="px-6 py-4 text-sm text-muted-foreground">
                {{ doc.quality_score ?? 'N/A' }}
              </td>
              <td class="px-6 py-4 text-sm text-muted-foreground">
                {{ formatDate(doc.crawled_at) }}
              </td>
              <td class="px-6 py-4">
                <div class="flex items-center justify-end gap-2">
                  <Button
                    variant="ghost"
                    size="sm"
                    @click="viewDocument(doc)"
                  >
                    <Eye class="h-4 w-4" />
                  </Button>
                  <a
                    :href="doc.url"
                    target="_blank"
                    class="text-primary hover:text-primary/80"
                  >
                    <ExternalLink class="h-4 w-4" />
                  </a>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </CardContent>
    </Card>

    <div
      v-if="classifiedIndexes.length === 0 && !loading && !error"
      class="text-center py-6"
    >
      <FileText class="h-12 w-12 text-muted-foreground mx-auto mb-4" />
      <p class="text-muted-foreground">
        No classified content indexes found. Content must be classified before it can be reviewed.
      </p>
    </div>
  </div>
</template>
