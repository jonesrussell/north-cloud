<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Loader2, Star } from 'lucide-vue-next'
import { classifierApi } from '@/api/client'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'

// Match the actual API response from classifier service
interface SourceReputation {
  name: string
  reputation: number          // 0-100 reputation score
  category: string            // news, blog, government, unknown
  total_classified: number    // total articles classified
  avg_quality: number         // average quality score
  last_updated: string | null
}

const loading = ref(true)
const error = ref<string | null>(null)
const sources = ref<SourceReputation[]>([])

const loadSources = async () => {
  try {
    loading.value = true
    error.value = null
    const response = await classifierApi.sources.list()
    sources.value = response.data?.sources || []
  } catch (err) {
    console.error('Failed to load sources:', err)
    error.value = 'Unable to load source reputation data.'
  } finally {
    loading.value = false
  }
}

const getScoreVariant = (score: number) => {
  if (score >= 80) return 'success'
  if (score >= 60) return 'warning'
  return 'destructive'
}

const formatDate = (date: string | null) => {
  if (!date) return 'Never'
  return new Date(date).toLocaleDateString()
}

onMounted(loadSources)
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-3xl font-bold tracking-tight">
        Source Reputation
      </h1>
      <p class="text-muted-foreground">
        Quality scores and performance metrics for content sources
      </p>
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

    <Card v-else-if="sources.length === 0">
      <CardContent class="flex flex-col items-center justify-center py-12">
        <Star class="h-12 w-12 text-muted-foreground mb-4" />
        <h3 class="text-lg font-medium mb-2">
          No reputation data
        </h3>
        <p class="text-muted-foreground">
          Source reputation will be calculated as content is classified.
        </p>
      </CardContent>
    </Card>

    <Card v-else>
      <CardHeader>
        <CardTitle>Source Quality Scores</CardTitle>
        <CardDescription>Based on content quality and classification results</CardDescription>
      </CardHeader>
      <CardContent class="p-0">
        <table class="w-full">
          <thead class="border-b bg-muted/50">
            <tr>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Source
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Category
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Reputation
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Avg Quality
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Total Classified
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Last Updated
              </th>
            </tr>
          </thead>
          <tbody class="divide-y">
            <tr
              v-for="source in sources"
              :key="source.name"
              class="hover:bg-muted/50"
            >
              <td class="px-6 py-4 text-sm font-medium">
                {{ source.name }}
              </td>
              <td class="px-6 py-4">
                <Badge variant="outline">
                  {{ source.category || 'unknown' }}
                </Badge>
              </td>
              <td class="px-6 py-4">
                <Badge :variant="getScoreVariant(source.reputation)">
                  {{ source.reputation }}/100
                </Badge>
              </td>
              <td class="px-6 py-4 text-sm text-muted-foreground">
                {{ source.avg_quality?.toFixed(1) || '0' }}
              </td>
              <td class="px-6 py-4 text-sm text-muted-foreground">
                {{ source.total_classified?.toLocaleString() || 0 }}
              </td>
              <td class="px-6 py-4 text-sm text-muted-foreground">
                {{ formatDate(source.last_updated) }}
              </td>
            </tr>
          </tbody>
        </table>
      </CardContent>
    </Card>
  </div>
</template>
