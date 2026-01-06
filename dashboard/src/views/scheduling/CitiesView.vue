<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Loader2, MapPin, Plus } from 'lucide-vue-next'
import { sourcesApi } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'

interface City {
  id: string
  name: string
  state: string
  country: string
  sources_count: number
}

const loading = ref(true)
const error = ref<string | null>(null)
const cities = ref<City[]>([])

const loadCities = async () => {
  try {
    loading.value = true
    const response = await sourcesApi.cities.list()
    cities.value = response.data?.cities || response.data || []
  } catch (err) {
    error.value = 'Unable to load cities.'
  } finally {
    loading.value = false
  }
}

onMounted(loadCities)
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-3xl font-bold tracking-tight">Cities</h1>
        <p class="text-muted-foreground">Geographic regions for content sources</p>
      </div>
      <Button>
        <Plus class="mr-2 h-4 w-4" />
        Add City
      </Button>
    </div>

    <div v-if="loading" class="flex items-center justify-center py-12">
      <Loader2 class="h-8 w-8 animate-spin text-muted-foreground" />
    </div>

    <Card v-else-if="error" class="border-destructive">
      <CardContent class="pt-6">
        <p class="text-destructive">{{ error }}</p>
      </CardContent>
    </Card>

    <Card v-else-if="cities.length === 0">
      <CardContent class="flex flex-col items-center justify-center py-12">
        <MapPin class="h-12 w-12 text-muted-foreground mb-4" />
        <h3 class="text-lg font-medium mb-2">No cities configured</h3>
        <p class="text-muted-foreground mb-4">Add cities to organize sources by location.</p>
        <Button>
          <Plus class="mr-2 h-4 w-4" />
          Add City
        </Button>
      </CardContent>
    </Card>

    <div v-else class="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
      <Card v-for="city in cities" :key="city.id" class="hover:shadow-md transition-shadow">
        <CardContent class="pt-6">
          <div class="flex items-start justify-between">
            <div>
              <h3 class="font-semibold">{{ city.name }}</h3>
              <p class="text-sm text-muted-foreground">{{ city.state }}, {{ city.country }}</p>
            </div>
            <MapPin class="h-5 w-5 text-muted-foreground" />
          </div>
          <p class="mt-4 text-sm text-muted-foreground">
            {{ city.sources_count || 0 }} sources
          </p>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
