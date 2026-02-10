<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { formatDate } from '@/lib/utils'
import { ArrowLeft, Loader2, ExternalLink, Copy, Check, RefreshCw } from 'lucide-vue-next'
import { indexManagerApi, classifierApi } from '@/api/client'
import type { Document } from '@/types/indexManager'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

interface ClassifyResult {
  content_type?: string
  quality_score?: number
  topics?: string[]
  crime?: { street_crime_relevance?: string; [key: string]: unknown }
  mining?: Record<string, unknown>
}

const route = useRoute()
const router = useRouter()

const indexName = computed(() => route.params.index_name as string)
const documentId = computed(() => route.params.document_id as string)
const isClassifiedIndex = computed(() =>
  (indexName.value ?? '').endsWith('_classified_content')
)

const loading = ref(true)
const error = ref<string | null>(null)
const document = ref<(Document & Record<string, unknown>) | null>(null)
const copied = ref(false)
const reclassifyLoading = ref(false)
const reclassifyError = ref<string | null>(null)

const loadDocument = async () => {
  try {
    loading.value = true
    error.value = null
    const response = await indexManagerApi.documents.get(indexName.value, documentId.value)
    document.value = response.data as Document & Record<string, unknown>
  } catch (err) {
    error.value = 'Unable to load document.'
  } finally {
    loading.value = false
  }
}

function applyResultToDocument(result: ClassifyResult): void {
  if (!document.value) return
  const doc = document.value as Record<string, unknown>
  if (result.content_type !== undefined) doc.content_type = result.content_type
  if (result.quality_score !== undefined) doc.quality_score = result.quality_score
  if (result.topics !== undefined) doc.topics = result.topics
  if (result.crime) {
    const existing = (doc.crime as Record<string, unknown>) ?? {}
    const rel = result.crime.street_crime_relevance ?? existing.relevance
    doc.crime = { ...existing, ...result.crime, relevance: rel }
  }
  if (result.mining) {
    const existing = (doc.mining as Record<string, unknown>) ?? {}
    doc.mining = { ...existing, ...result.mining }
  }
}

const handleReclassify = async () => {
  if (!documentId.value) return
  reclassifyLoading.value = true
  reclassifyError.value = null
  try {
    const response = await classifierApi.classify.reclassify(documentId.value)
    const data = response.data as { result?: ClassifyResult }
    if (data.result) {
      applyResultToDocument(data.result)
    }
    await loadDocument()
  } catch (err: unknown) {
    const axiosErr = err as { response?: { status?: number; data?: { error?: string } } }
    if (axiosErr?.response?.status === 404) {
      reclassifyError.value = 'Document not found.'
    } else {
      const msg = axiosErr?.response?.data?.error ?? (err as Error)?.message ?? 'Reclassification failed.'
      reclassifyError.value = msg
    }
  } finally {
    reclassifyLoading.value = false
  }
}

const copyJson = async () => {
  await navigator.clipboard.writeText(JSON.stringify(document.value, null, 2))
  copied.value = true
  setTimeout(() => (copied.value = false), 2000)
}

onMounted(loadDocument)
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center gap-4">
      <Button
        variant="ghost"
        size="icon"
        @click="router.push(`/intelligence/indexes/${indexName}`)"
      >
        <ArrowLeft class="h-5 w-5" />
      </Button>
      <div>
        <h1 class="text-3xl font-bold tracking-tight">
          Document Details
        </h1>
        <p class="text-muted-foreground">
          {{ indexName }} / {{ documentId }}
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

    <template v-else-if="document">
      <!-- Document Info -->
      <Card>
        <CardHeader>
          <CardTitle>{{ (document as Record<string, unknown>).title || 'Untitled Document' }}</CardTitle>
        </CardHeader>
        <CardContent>
          <dl class="grid grid-cols-2 gap-4">
            <div v-if="(document as Record<string, unknown>).url">
              <dt class="text-sm text-muted-foreground">
                URL
              </dt>
              <dd class="mt-1">
                <a 
                  :href="(document as Record<string, unknown>).url as string" 
                  target="_blank" 
                  class="text-primary hover:underline flex items-center gap-1"
                >
                  {{ (document as Record<string, unknown>).url }}
                  <ExternalLink class="h-3 w-3" />
                </a>
              </dd>
            </div>
            <div v-if="(document as Record<string, unknown>).content_type">
              <dt class="text-sm text-muted-foreground">
                Content Type
              </dt>
              <dd class="mt-1">
                <Badge variant="secondary">
                  {{ (document as Record<string, unknown>).content_type }}
                </Badge>
              </dd>
            </div>
            <div v-if="(document as Record<string, unknown>).quality_score">
              <dt class="text-sm text-muted-foreground">
                Quality Score
              </dt>
              <dd class="mt-1">
                <Badge variant="outline">
                  {{ (document as Record<string, unknown>).quality_score }}/100
                </Badge>
              </dd>
            </div>
            <div v-if="(document as Record<string, unknown>).is_crime_related !== undefined">
              <dt class="text-sm text-muted-foreground">
                Crime Related
              </dt>
              <dd class="mt-1">
                <Badge :variant="(document as Record<string, unknown>).is_crime_related ? 'destructive' : 'secondary'">
                  {{ (document as Record<string, unknown>).is_crime_related ? 'Yes' : 'No' }}
                </Badge>
              </dd>
            </div>
            <div
              v-if="(document as Record<string, unknown>).topics && Array.isArray((document as Record<string, unknown>).topics) && ((document as Record<string, unknown>).topics as string[]).length > 0"
              class="col-span-2"
            >
              <dt class="text-sm text-muted-foreground">
                Topics
              </dt>
              <dd class="mt-1 flex flex-wrap gap-2">
                <Badge
                  v-for="topic in (document as Record<string, unknown>).topics as string[]"
                  :key="topic"
                  variant="secondary"
                >
                  {{ topic }}
                </Badge>
              </dd>
            </div>
            <div v-if="(document as Record<string, unknown>).published_date">
              <dt class="text-sm text-muted-foreground">
                Published
              </dt>
              <dd class="mt-1 text-sm">
                {{ formatDate((document as Record<string, unknown>).published_date as string) }}
              </dd>
            </div>
            <div v-if="(document as Record<string, unknown>).created_at">
              <dt class="text-sm text-muted-foreground">
                Indexed
              </dt>
              <dd class="mt-1 text-sm">
                {{ formatDate((document as Record<string, unknown>).created_at as string) }}
              </dd>
            </div>
          </dl>
        </CardContent>
      </Card>

      <!-- Crime Decision Audit -->
      <Card v-if="document.crime">
        <CardHeader>
          <CardTitle class="text-lg">
            Crime Classification Audit
          </CardTitle>
        </CardHeader>
        <CardContent>
          <dl class="grid grid-cols-2 gap-4">
            <div v-if="document.crime.relevance">
              <dt class="text-sm text-muted-foreground">
                Relevance
              </dt>
              <dd class="mt-1">
                <Badge variant="destructive">
                  {{ document.crime.relevance }}
                </Badge>
              </dd>
            </div>
            <div v-if="document.crime.confidence !== undefined">
              <dt class="text-sm text-muted-foreground">
                Confidence
              </dt>
              <dd class="mt-1 font-mono text-sm">
                {{ document.crime.confidence?.toFixed(2) ?? 'N/A' }}
              </dd>
            </div>
            <div v-if="document.crime.sub_label">
              <dt class="text-sm text-muted-foreground">
                Sub Label
              </dt>
              <dd class="mt-1">
                <Badge variant="secondary">
                  {{ document.crime.sub_label }}
                </Badge>
              </dd>
            </div>
            <div v-if="document.crime.homepage_eligible !== undefined">
              <dt class="text-sm text-muted-foreground">
                Homepage Eligible
              </dt>
              <dd class="mt-1">
                <Badge :variant="document.crime.homepage_eligible ? 'default' : 'secondary'">
                  {{ document.crime.homepage_eligible ? 'Yes' : 'No' }}
                </Badge>
              </dd>
            </div>
            <div v-if="document.crime.review_required !== undefined">
              <dt class="text-sm text-muted-foreground">
                Review Required
              </dt>
              <dd class="mt-1">
                <Badge :variant="document.crime.review_required ? 'destructive' : 'secondary'">
                  {{ document.crime.review_required ? 'Yes' : 'No' }}
                </Badge>
              </dd>
            </div>
            <div v-if="document.crime.crime_types?.length">
              <dt class="text-sm text-muted-foreground">
                Crime Types
              </dt>
              <dd class="mt-1 flex flex-wrap gap-1">
                <Badge
                  v-for="ct in document.crime.crime_types"
                  :key="ct"
                  variant="outline"
                  class="text-xs"
                >
                  {{ ct }}
                </Badge>
              </dd>
            </div>
            <div v-if="document.crime.model_version">
              <dt class="text-sm text-muted-foreground">
                Model Version
              </dt>
              <dd class="mt-1 font-mono text-xs text-muted-foreground">
                {{ document.crime.model_version }}
              </dd>
            </div>
          </dl>
        </CardContent>
      </Card>

      <!-- Mining Decision Audit -->
      <Card v-if="document.mining">
        <CardHeader>
          <CardTitle class="text-lg">
            Mining Classification Audit
          </CardTitle>
        </CardHeader>
        <CardContent>
          <dl class="grid grid-cols-2 gap-4">
            <div v-if="document.mining.relevance">
              <dt class="text-sm text-muted-foreground">
                Relevance
              </dt>
              <dd class="mt-1">
                <Badge
                  variant="default"
                  class="bg-amber-500"
                >
                  {{ document.mining.relevance }}
                </Badge>
              </dd>
            </div>
            <div v-if="document.mining.final_confidence !== undefined">
              <dt class="text-sm text-muted-foreground">
                Confidence
              </dt>
              <dd class="mt-1 font-mono text-sm">
                {{ document.mining.final_confidence?.toFixed(2) ?? 'N/A' }}
              </dd>
            </div>
            <div v-if="document.mining.mining_stage">
              <dt class="text-sm text-muted-foreground">
                Mining Stage
              </dt>
              <dd class="mt-1">
                <Badge variant="secondary">
                  {{ document.mining.mining_stage }}
                </Badge>
              </dd>
            </div>
            <div v-if="document.mining.review_required !== undefined">
              <dt class="text-sm text-muted-foreground">
                Review Required
              </dt>
              <dd class="mt-1">
                <Badge :variant="document.mining.review_required ? 'destructive' : 'secondary'">
                  {{ document.mining.review_required ? 'Yes' : 'No' }}
                </Badge>
              </dd>
            </div>
            <div v-if="document.mining.commodities?.length">
              <dt class="text-sm text-muted-foreground">
                Commodities
              </dt>
              <dd class="mt-1 flex flex-wrap gap-1">
                <Badge
                  v-for="c in document.mining.commodities"
                  :key="c"
                  variant="outline"
                  class="text-xs"
                >
                  {{ c }}
                </Badge>
              </dd>
            </div>
            <div v-if="document.mining.model_version">
              <dt class="text-sm text-muted-foreground">
                Model Version
              </dt>
              <dd class="mt-1 font-mono text-xs text-muted-foreground">
                {{ document.mining.model_version }}
              </dd>
            </div>
          </dl>
        </CardContent>
      </Card>

      <!-- Raw JSON -->
      <Card>
        <CardHeader>
          <div class="flex flex-col gap-2">
            <div class="flex items-center justify-between">
              <CardTitle>Raw JSON</CardTitle>
              <div class="flex items-center gap-2">
                <Button
                  v-if="isClassifiedIndex"
                  variant="outline"
                  size="sm"
                  :disabled="reclassifyLoading"
                  @click="handleReclassify"
                >
                  <Loader2
                    v-if="reclassifyLoading"
                    class="mr-2 h-4 w-4 animate-spin"
                  />
                  <RefreshCw
                    v-else
                    class="mr-2 h-4 w-4"
                  />
                  {{ reclassifyLoading ? 'Reclassifying…' : 'Reclassify' }}
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  @click="copyJson"
                >
                  <Check
                    v-if="copied"
                    class="mr-2 h-4 w-4"
                  />
                  <Copy
                    v-else
                    class="mr-2 h-4 w-4"
                  />
                  {{ copied ? 'Copied!' : 'Copy' }}
                </Button>
              </div>
            </div>
            <p
              v-if="reclassifyError"
              class="text-sm text-destructive"
            >
              {{ reclassifyError }}
            </p>
          </div>
        </CardHeader>
        <CardContent>
          <pre class="bg-muted p-4 rounded-lg overflow-auto text-xs max-h-96">{{ JSON.stringify(document, null, 2) }}</pre>
        </CardContent>
      </Card>
    </template>
  </div>
</template>
