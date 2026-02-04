<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { Loader2, MapPin, RefreshCw, Globe, Building2, Map } from 'lucide-vue-next'
import { indexManagerApi } from '@/api/client'
import type { LocationAggregation } from '@/types/aggregation'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'

const loading = ref(true)
const error = ref<string | null>(null)
const aggregation = ref<LocationAggregation | null>(null)

const loadAggregation = async () => {
  try {
    loading.value = true
    error.value = null
    const response = await indexManagerApi.aggregations.getLocation()
    aggregation.value = response.data
  } catch (err) {
    error.value = 'Unable to load location aggregation data.'
    console.error('Failed to load location aggregation:', err)
  } finally {
    loading.value = false
  }
}

// Convert map to sorted array for display
const countryData = computed(() => {
  if (!aggregation.value?.by_country) return []
  return Object.entries(aggregation.value.by_country)
    .map(([name, count]) => ({ name, count }))
    .sort((a, b) => b.count - a.count)
})

const provinceData = computed(() => {
  if (!aggregation.value?.by_province) return []
  return Object.entries(aggregation.value.by_province)
    .map(([name, count]) => ({ name, count }))
    .sort((a, b) => b.count - a.count)
})

const cityData = computed(() => {
  if (!aggregation.value?.by_city) return []
  return Object.entries(aggregation.value.by_city)
    .map(([name, count]) => ({ name, count }))
    .sort((a, b) => b.count - a.count)
    .slice(0, 20) // Limit to top 20 cities
})

const specificityData = computed(() => {
  if (!aggregation.value?.by_specificity) return []
  return Object.entries(aggregation.value.by_specificity)
    .map(([name, count]) => ({ name: formatLabel(name), count }))
    .sort((a, b) => b.count - a.count)
})

const totalLocations = computed(() => {
  if (!aggregation.value?.by_city) return 0
  return Object.values(aggregation.value.by_city).reduce((sum, count) => sum + count, 0)
})

const uniqueCities = computed(() => {
  if (!aggregation.value?.by_city) return 0
  return Object.keys(aggregation.value.by_city).length
})

const uniqueProvinces = computed(() => {
  if (!aggregation.value?.by_province) return 0
  return Object.keys(aggregation.value.by_province).length
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

const getSpecificityColor = (spec: string) => {
  const lower = spec.toLowerCase()
  if (lower.includes('city')) return 'bg-green-500'
  if (lower.includes('province') || lower.includes('state')) return 'bg-blue-500'
  if (lower.includes('region')) return 'bg-purple-500'
  if (lower.includes('country')) return 'bg-orange-500'
  return 'bg-gray-400'
}

onMounted(loadAggregation)
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-3xl font-bold tracking-tight">
          Location Breakdown
        </h1>
        <p class="text-muted-foreground">
          Geographic distribution of content across all indexes
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
            <CardDescription>Documents with Location</CardDescription>
            <CardTitle class="text-3xl">
              {{ formatNumber(totalLocations) }}
            </CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader class="pb-2">
            <CardDescription>Unique Cities</CardDescription>
            <CardTitle class="text-3xl text-green-500">
              {{ formatNumber(uniqueCities) }}
            </CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader class="pb-2">
            <CardDescription>Provinces/States</CardDescription>
            <CardTitle class="text-3xl text-blue-500">
              {{ formatNumber(uniqueProvinces) }}
            </CardTitle>
          </CardHeader>
        </Card>
      </div>

      <!-- Charts Grid -->
      <div class="grid gap-6 lg:grid-cols-2">
        <!-- By Country -->
        <Card>
          <CardHeader>
            <CardTitle class="flex items-center gap-2">
              <Globe class="h-5 w-5" />
              By Country
            </CardTitle>
            <CardDescription>
              Content distribution by country
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div
              v-if="countryData.length === 0"
              class="text-center py-8 text-muted-foreground"
            >
              No data available
            </div>
            <div
              v-else
              class="space-y-3"
            >
              <div
                v-for="item in countryData"
                :key="item.name"
                class="space-y-1"
              >
                <div class="flex justify-between text-sm">
                  <span>{{ item.name }}</span>
                  <span class="font-medium">{{ formatNumber(item.count) }}</span>
                </div>
                <div class="h-2 bg-muted rounded-full overflow-hidden">
                  <div
                    class="h-full bg-orange-500 rounded-full transition-all"
                    :style="{ width: `${getBarWidth(item.count, countryData)}%` }"
                  />
                </div>
              </div>
            </div>
          </CardContent>
        </Card>

        <!-- By Province -->
        <Card>
          <CardHeader>
            <CardTitle class="flex items-center gap-2">
              <Map class="h-5 w-5" />
              By Province/State
            </CardTitle>
            <CardDescription>
              Content distribution by province or state
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div
              v-if="provinceData.length === 0"
              class="text-center py-8 text-muted-foreground"
            >
              No data available
            </div>
            <div
              v-else
              class="space-y-3"
            >
              <div
                v-for="item in provinceData.slice(0, 10)"
                :key="item.name"
                class="space-y-1"
              >
                <div class="flex justify-between text-sm">
                  <span>{{ item.name }}</span>
                  <span class="font-medium">{{ formatNumber(item.count) }}</span>
                </div>
                <div class="h-2 bg-muted rounded-full overflow-hidden">
                  <div
                    class="h-full bg-blue-500 rounded-full transition-all"
                    :style="{ width: `${getBarWidth(item.count, provinceData)}%` }"
                  />
                </div>
              </div>
            </div>
          </CardContent>
        </Card>

        <!-- By City -->
        <Card class="lg:col-span-2">
          <CardHeader>
            <CardTitle class="flex items-center gap-2">
              <Building2 class="h-5 w-5" />
              Top Cities
            </CardTitle>
            <CardDescription>
              Top 20 cities by content volume
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div
              v-if="cityData.length === 0"
              class="text-center py-8 text-muted-foreground"
            >
              No data available
            </div>
            <div
              v-else
              class="grid gap-2 sm:grid-cols-2 lg:grid-cols-4"
            >
              <div
                v-for="(item, index) in cityData"
                :key="item.name"
                class="flex items-center justify-between p-2 rounded-md bg-muted/50"
              >
                <div class="flex items-center gap-2">
                  <span class="text-muted-foreground text-sm w-5">{{ index + 1 }}.</span>
                  <span class="font-medium text-sm truncate">{{ item.name }}</span>
                </div>
                <Badge variant="secondary">
                  {{ formatNumber(item.count) }}
                </Badge>
              </div>
            </div>
          </CardContent>
        </Card>

        <!-- By Specificity -->
        <Card class="lg:col-span-2">
          <CardHeader>
            <CardTitle class="flex items-center gap-2">
              <MapPin class="h-5 w-5" />
              Location Specificity
            </CardTitle>
            <CardDescription>
              How specific are the detected locations
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div
              v-if="specificityData.length === 0"
              class="text-center py-8 text-muted-foreground"
            >
              No data available
            </div>
            <div
              v-else
              class="flex flex-wrap gap-4"
            >
              <div
                v-for="item in specificityData"
                :key="item.name"
                class="flex items-center gap-2"
              >
                <div
                  class="w-3 h-3 rounded-full"
                  :class="getSpecificityColor(item.name)"
                />
                <span class="text-sm">{{ item.name }}</span>
                <Badge variant="outline">
                  {{ formatNumber(item.count) }}
                </Badge>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </template>
  </div>
</template>
