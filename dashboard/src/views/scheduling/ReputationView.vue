<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Loader2, Star, TrendingUp, TrendingDown } from 'lucide-vue-next'
import { classifierApi } from '@/api/client'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'

interface SourceReputation {
  name: string
  quality_score: number
  total_articles: number
  crime_rate: number
  trend: 'up' | 'down' | 'stable'
}

const loading = ref(true)
const error = ref<string | null>(null)
const sources = ref<SourceReputation[]>([])

const loadSources = async () => {
  try {
    loading.value = true
    const response = await classifierApi.sources.list()
    sources.value = response.data?.sources || response.data || []
  } catch (err) {
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
                Quality Score
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Total Articles
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Crime Rate
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Trend
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
                <Badge :variant="getScoreVariant(source.quality_score)">
                  {{ source.quality_score }}/100
                </Badge>
              </td>
              <td class="px-6 py-4 text-sm text-muted-foreground">
                {{ source.total_articles.toLocaleString() }}
              </td>
              <td class="px-6 py-4 text-sm text-muted-foreground">
                {{ (source.crime_rate * 100).toFixed(1) }}%
              </td>
              <td class="px-6 py-4">
                <TrendingUp
                  v-if="source.trend === 'up'"
                  class="h-4 w-4 text-green-500"
                />
                <TrendingDown
                  v-else-if="source.trend === 'down'"
                  class="h-4 w-4 text-red-500"
                />
                <span
                  v-else
                  class="text-muted-foreground"
                >—</span>
              </td>
            </tr>
          </tbody>
        </table>
      </CardContent>
    </Card>
  </div>
</template>
