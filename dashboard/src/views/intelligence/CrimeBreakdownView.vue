<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { Loader2, AlertTriangle, RefreshCw, BarChart3 } from 'lucide-vue-next'
import { indexManagerApi } from '@/api/client'
import type { CrimeAggregation } from '@/types/aggregation'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'

const loading = ref(true)
const error = ref<string | null>(null)
const aggregation = ref<CrimeAggregation | null>(null)

const loadAggregation = async () => {
  try {
    loading.value = true
    error.value = null
    const response = await indexManagerApi.aggregations.getCrime()
    aggregation.value = response.data
  } catch (err) {
    error.value = 'Unable to load crime aggregation data.'
    console.error('Failed to load crime aggregation:', err)
  } finally {
    loading.value = false
  }
}

const crimePercentage = computed(() => {
  if (!aggregation.value) return 0
  const { total_crime_related, total_documents } = aggregation.value
  if (total_documents === 0) return 0
  return Math.round((total_crime_related / total_documents) * 100)
})

// Convert map to sorted array for display
const subLabelData = computed(() => {
  if (!aggregation.value?.by_sub_label) return []
  return Object.entries(aggregation.value.by_sub_label)
    .map(([name, count]) => ({ name: formatLabel(name), count }))
    .sort((a, b) => b.count - a.count)
})

const relevanceData = computed(() => {
  if (!aggregation.value?.by_relevance) return []
  return Object.entries(aggregation.value.by_relevance)
    .map(([name, count]) => ({ name: formatLabel(name), count }))
    .sort((a, b) => b.count - a.count)
})

const crimeTypeData = computed(() => {
  if (!aggregation.value?.by_crime_type) return []
  return Object.entries(aggregation.value.by_crime_type)
    .map(([name, count]) => ({ name: formatLabel(name), count }))
    .sort((a, b) => b.count - a.count)
})

const formatLabel = (label: string) => {
  return label
    .replace(/_/g, ' ')
    .replace(/\b\w/g, (c) => c.toUpperCase())
}

const formatNumber = (num: number) => {
  return num.toLocaleString()
}

// Calculate bar width for simple bar chart
const getBarWidth = (count: number, data: { count: number }[]) => {
  if (data.length === 0) return 0
  const max = Math.max(...data.map((d) => d.count))
  if (max === 0) return 0
  return (count / max) * 100
}

const getRelevanceColor = (relevance: string) => {
  const lower = relevance.toLowerCase()
  if (lower.includes('direct')) return 'bg-red-500'
  if (lower.includes('related')) return 'bg-yellow-500'
  if (lower.includes('peripheral')) return 'bg-blue-400'
  return 'bg-gray-400'
}

const getSubLabelColor = (label: string) => {
  const lower = label.toLowerCase()
  if (lower.includes('violent')) return 'bg-red-500'
  if (lower.includes('property')) return 'bg-orange-500'
  if (lower.includes('drug')) return 'bg-purple-500'
  if (lower.includes('organized')) return 'bg-yellow-600'
  if (lower.includes('justice')) return 'bg-blue-500'
  return 'bg-gray-400'
}

onMounted(loadAggregation)
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-3xl font-bold tracking-tight">
          Crime Breakdown
        </h1>
        <p class="text-muted-foreground">
          Distribution of crime-related content across all indexes
        </p>
      </div>
      <Button
        variant="outline"
        :disabled="loading"
        @click="loadAggregation"
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

    <template v-else-if="aggregation">
      <!-- Summary Cards -->
      <div class="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader class="pb-2">
            <CardDescription>Total Documents</CardDescription>
            <CardTitle class="text-3xl">
              {{ formatNumber(aggregation.total_documents) }}
            </CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader class="pb-2">
            <CardDescription>Crime Related</CardDescription>
            <CardTitle class="text-3xl text-red-500">
              {{ formatNumber(aggregation.total_crime_related) }}
            </CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader class="pb-2">
            <CardDescription>Crime Percentage</CardDescription>
            <CardTitle class="text-3xl">
              {{ crimePercentage }}%
            </CardTitle>
          </CardHeader>
        </Card>
      </div>

      <!-- Charts Grid -->
      <div class="grid gap-6 lg:grid-cols-2">
        <!-- By Sub-Label -->
        <Card>
          <CardHeader>
            <CardTitle class="flex items-center gap-2">
              <AlertTriangle class="h-5 w-5" />
              By Category
            </CardTitle>
            <CardDescription>
              Crime sub-categories distribution
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div
              v-if="subLabelData.length === 0"
              class="text-center py-8 text-muted-foreground"
            >
              No data available
            </div>
            <div
              v-else
              class="space-y-3"
            >
              <div
                v-for="item in subLabelData"
                :key="item.name"
                class="space-y-1"
              >
                <div class="flex justify-between text-sm">
                  <span>{{ item.name }}</span>
                  <span class="font-medium">{{ formatNumber(item.count) }}</span>
                </div>
                <div class="h-2 bg-muted rounded-full overflow-hidden">
                  <div
                    class="h-full rounded-full transition-all"
                    :class="getSubLabelColor(item.name)"
                    :style="{ width: `${getBarWidth(item.count, subLabelData)}%` }"
                  />
                </div>
              </div>
            </div>
          </CardContent>
        </Card>

        <!-- By Relevance -->
        <Card>
          <CardHeader>
            <CardTitle class="flex items-center gap-2">
              <BarChart3 class="h-5 w-5" />
              By Relevance
            </CardTitle>
            <CardDescription>
              How directly related to crime
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div
              v-if="relevanceData.length === 0"
              class="text-center py-8 text-muted-foreground"
            >
              No data available
            </div>
            <div
              v-else
              class="space-y-3"
            >
              <div
                v-for="item in relevanceData"
                :key="item.name"
                class="space-y-1"
              >
                <div class="flex justify-between text-sm">
                  <span>{{ item.name }}</span>
                  <span class="font-medium">{{ formatNumber(item.count) }}</span>
                </div>
                <div class="h-2 bg-muted rounded-full overflow-hidden">
                  <div
                    class="h-full rounded-full transition-all"
                    :class="getRelevanceColor(item.name)"
                    :style="{ width: `${getBarWidth(item.count, relevanceData)}%` }"
                  />
                </div>
              </div>
            </div>
          </CardContent>
        </Card>

        <!-- By Crime Type -->
        <Card class="lg:col-span-2">
          <CardHeader>
            <CardTitle>By Crime Type</CardTitle>
            <CardDescription>
              Specific crime types identified in content
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div
              v-if="crimeTypeData.length === 0"
              class="text-center py-8 text-muted-foreground"
            >
              No data available
            </div>
            <div
              v-else
              class="flex flex-wrap gap-2"
            >
              <Badge
                v-for="item in crimeTypeData"
                :key="item.name"
                variant="secondary"
                class="text-sm"
              >
                {{ item.name }}
                <span class="ml-1 text-muted-foreground">({{ formatNumber(item.count) }})</span>
              </Badge>
            </div>
          </CardContent>
        </Card>
      </div>
    </template>
  </div>
</template>
