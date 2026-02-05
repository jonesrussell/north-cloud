<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { Loader2, Pickaxe, RefreshCw, BarChart3, MapPin } from 'lucide-vue-next'
import { indexManagerApi } from '@/api/client'
import type { MiningAggregation } from '@/types/aggregation'
import { ClassifierHealthWidget } from '@/components/domain/classifier'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'

const loading = ref(true)
const error = ref<string | null>(null)
const aggregation = ref<MiningAggregation | null>(null)

const loadAggregation = async () => {
  try {
    loading.value = true
    error.value = null
    const response = await indexManagerApi.aggregations.getMining()
    aggregation.value = response.data
  } catch (err) {
    error.value = 'Unable to load mining aggregation data.'
    console.error('Failed to load mining aggregation:', err)
  } finally {
    loading.value = false
  }
}

const miningPercentage = computed(() => {
  if (!aggregation.value) return 0
  const { total_mining, total_documents } = aggregation.value
  if (total_documents === 0) return 0
  return Math.round((total_mining / total_documents) * 100)
})

const relevanceData = computed(() => {
  if (!aggregation.value?.by_relevance) return []
  return Object.entries(aggregation.value.by_relevance)
    .map(([name, count]) => ({ name: formatLabel(name), count }))
    .sort((a, b) => b.count - a.count)
})

const stageData = computed(() => {
  if (!aggregation.value?.by_mining_stage) return []
  return Object.entries(aggregation.value.by_mining_stage)
    .map(([name, count]) => ({ name: formatLabel(name), count }))
    .sort((a, b) => b.count - a.count)
})

const commodityData = computed(() => {
  if (!aggregation.value?.by_commodity) return []
  return Object.entries(aggregation.value.by_commodity)
    .map(([name, count]) => ({ name: formatLabel(name), count }))
    .sort((a, b) => b.count - a.count)
})

const locationData = computed(() => {
  if (!aggregation.value?.by_location) return []
  return Object.entries(aggregation.value.by_location)
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

const getBarWidth = (count: number, data: { count: number }[]) => {
  if (data.length === 0) return 0
  const max = Math.max(...data.map((d) => d.count))
  if (max === 0) return 0
  return (count / max) * 100
}

const getRelevanceColor = (relevance: string) => {
  const lower = relevance.toLowerCase()
  if (lower.includes('core')) return 'bg-amber-500'
  if (lower.includes('peripheral')) return 'bg-yellow-400'
  if (lower.includes('not')) return 'bg-gray-400'
  return 'bg-gray-400'
}

const getStageColor = (stage: string) => {
  const lower = stage.toLowerCase()
  if (lower.includes('exploration')) return 'bg-sky-500'
  if (lower.includes('development')) return 'bg-blue-600'
  if (lower.includes('production')) return 'bg-green-500'
  if (lower.includes('unspecified')) return 'bg-gray-400'
  return 'bg-gray-400'
}

const getCommodityColor = (commodity: string) => {
  const lower = commodity.toLowerCase()
  if (lower.includes('gold')) return 'bg-yellow-500'
  if (lower.includes('copper')) return 'bg-orange-500'
  if (lower.includes('lithium')) return 'bg-cyan-500'
  if (lower.includes('nickel')) return 'bg-slate-500'
  if (lower.includes('uranium')) return 'bg-lime-500'
  if (lower.includes('iron')) return 'bg-red-700'
  if (lower.includes('rare')) return 'bg-purple-500'
  return 'bg-gray-400'
}

onMounted(loadAggregation)
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-start justify-between gap-4">
      <div>
        <h1 class="text-3xl font-bold tracking-tight">
          Mining Breakdown
        </h1>
        <p class="text-muted-foreground">
          Distribution of mining-related content across all indexes
        </p>
      </div>
      <div class="flex items-center gap-2">
        <ClassifierHealthWidget />
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
            <CardDescription>Mining Related</CardDescription>
            <CardTitle class="text-3xl text-amber-500">
              {{ formatNumber(aggregation.total_mining) }}
            </CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader class="pb-2">
            <CardDescription>Mining Percentage</CardDescription>
            <CardTitle class="text-3xl">
              {{ miningPercentage }}%
            </CardTitle>
          </CardHeader>
        </Card>
      </div>

      <!-- Charts Grid -->
      <div class="grid gap-6 lg:grid-cols-2">
        <!-- By Relevance -->
        <Card>
          <CardHeader>
            <CardTitle class="flex items-center gap-2">
              <Pickaxe class="h-5 w-5" />
              By Relevance
            </CardTitle>
            <CardDescription>
              Mining content relevance classification
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

        <!-- By Mining Stage -->
        <Card>
          <CardHeader>
            <CardTitle class="flex items-center gap-2">
              <BarChart3 class="h-5 w-5" />
              By Mining Stage
            </CardTitle>
            <CardDescription>
              Lifecycle stage of mining content
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div
              v-if="stageData.length === 0"
              class="text-center py-8 text-muted-foreground"
            >
              No data available
            </div>
            <div
              v-else
              class="space-y-3"
            >
              <div
                v-for="item in stageData"
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
                    :class="getStageColor(item.name)"
                    :style="{ width: `${getBarWidth(item.count, stageData)}%` }"
                  />
                </div>
              </div>
            </div>
          </CardContent>
        </Card>

        <!-- By Commodity -->
        <Card>
          <CardHeader>
            <CardTitle class="flex items-center gap-2">
              <Pickaxe class="h-5 w-5" />
              By Commodity
            </CardTitle>
            <CardDescription>
              Commodities mentioned in mining content
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div
              v-if="commodityData.length === 0"
              class="text-center py-8 text-muted-foreground"
            >
              No data available
            </div>
            <div
              v-else
              class="space-y-3"
            >
              <div
                v-for="item in commodityData"
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
                    :class="getCommodityColor(item.name)"
                    :style="{ width: `${getBarWidth(item.count, commodityData)}%` }"
                  />
                </div>
              </div>
            </div>
          </CardContent>
        </Card>

        <!-- By Location -->
        <Card>
          <CardHeader>
            <CardTitle class="flex items-center gap-2">
              <MapPin class="h-5 w-5" />
              By Location
            </CardTitle>
            <CardDescription>
              Geographic distribution of mining content
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div
              v-if="locationData.length === 0"
              class="text-center py-8 text-muted-foreground"
            >
              No data available
            </div>
            <div
              v-else
              class="space-y-3"
            >
              <div
                v-for="item in locationData"
                :key="item.name"
                class="space-y-1"
              >
                <div class="flex justify-between text-sm">
                  <span>{{ item.name }}</span>
                  <span class="font-medium">{{ formatNumber(item.count) }}</span>
                </div>
                <div class="h-2 bg-muted rounded-full overflow-hidden">
                  <div
                    class="h-full rounded-full transition-all bg-emerald-500"
                    :style="{ width: `${getBarWidth(item.count, locationData)}%` }"
                  />
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </template>
  </div>
</template>
