<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Loader2, HardDrive, RefreshCw, Database, Trash2 } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'

interface CacheStats {
  used_memory: string
  total_keys: number
  connected_clients: number
  uptime_days: number
  hit_rate: number
}

const loading = ref(true)
const error = ref<string | null>(null)
const stats = ref<CacheStats | null>(null)

const loadStats = async () => {
  try {
    loading.value = true
    // Mock data - in production, this would come from a Redis stats API
    await new Promise((resolve) => setTimeout(resolve, 500))
    stats.value = {
      used_memory: '128.5 MB',
      total_keys: 15420,
      connected_clients: 8,
      uptime_days: 45,
      hit_rate: 94.2,
    }
  } catch (err) {
    error.value = 'Unable to load cache statistics.'
  } finally {
    loading.value = false
  }
}

const formatNumber = (num: number) => num.toLocaleString()

onMounted(loadStats)
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-3xl font-bold tracking-tight">
          Cache Status
        </h1>
        <p class="text-muted-foreground">
          Redis cache statistics and management
        </p>
      </div>
      <Button
        variant="outline"
        @click="loadStats"
      >
        <RefreshCw class="mr-2 h-4 w-4" />
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

    <template v-else-if="stats">
      <!-- Stats Grid -->
      <div class="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardContent class="pt-6">
            <div class="flex items-center gap-4">
              <div class="p-2 bg-primary/10 rounded-lg">
                <HardDrive class="h-5 w-5 text-primary" />
              </div>
              <div>
                <p class="text-sm text-muted-foreground">
                  Memory Used
                </p>
                <p class="text-2xl font-bold">
                  {{ stats.used_memory }}
                </p>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent class="pt-6">
            <div class="flex items-center gap-4">
              <div class="p-2 bg-primary/10 rounded-lg">
                <Database class="h-5 w-5 text-primary" />
              </div>
              <div>
                <p class="text-sm text-muted-foreground">
                  Total Keys
                </p>
                <p class="text-2xl font-bold">
                  {{ formatNumber(stats.total_keys) }}
                </p>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent class="pt-6">
            <div class="flex items-center gap-4">
              <div class="p-2 bg-green-500/10 rounded-lg">
                <svg
                  class="h-5 w-5 text-green-500"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6"
                  />
                </svg>
              </div>
              <div>
                <p class="text-sm text-muted-foreground">
                  Hit Rate
                </p>
                <p class="text-2xl font-bold">
                  {{ stats.hit_rate }}%
                </p>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent class="pt-6">
            <div class="flex items-center gap-4">
              <div class="p-2 bg-primary/10 rounded-lg">
                <svg
                  class="h-5 w-5 text-primary"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
                  />
                </svg>
              </div>
              <div>
                <p class="text-sm text-muted-foreground">
                  Uptime
                </p>
                <p class="text-2xl font-bold">
                  {{ stats.uptime_days }} days
                </p>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      <!-- Cache Management -->
      <Card>
        <CardHeader>
          <CardTitle>Cache Management</CardTitle>
          <CardDescription>Administrative actions for Redis cache</CardDescription>
        </CardHeader>
        <CardContent>
          <div class="space-y-4">
            <div class="flex items-center justify-between p-4 border rounded-lg">
              <div>
                <p class="font-medium">
                  Clear Publish History Cache
                </p>
                <p class="text-sm text-muted-foreground">
                  Remove cached publish history data
                </p>
              </div>
              <Button
                variant="outline"
                size="sm"
              >
                <Trash2 class="mr-2 h-4 w-4" />
                Clear
              </Button>
            </div>
            <div class="flex items-center justify-between p-4 border rounded-lg">
              <div>
                <p class="font-medium">
                  Clear Article Cache
                </p>
                <p class="text-sm text-muted-foreground">
                  Remove cached article data
                </p>
              </div>
              <Button
                variant="outline"
                size="sm"
              >
                <Trash2 class="mr-2 h-4 w-4" />
                Clear
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>
    </template>
  </div>
</template>
