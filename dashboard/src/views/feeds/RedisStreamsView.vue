<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Loader2, Activity, RefreshCw } from 'lucide-vue-next'
import { publisherApi } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import type { ActiveChannel } from '@/types/publisher'

interface StreamStats {
  name: string
  messages_count: number
  last_activity: string | null
  status: 'active' | 'idle' | 'never'
  enabled: boolean
}

const loading = ref(true)
const error = ref<string | null>(null)
const streams = ref<StreamStats[]>([])

const loadStreams = async () => {
  try {
    loading.value = true
    error.value = null
    const response = await publisherApi.stats.activeChannels()
    const channels: ActiveChannel[] = response.data?.channels || []
    
    streams.value = channels.map((ch) => ({
      name: ch.name,
      messages_count: ch.total_published || 0,
      last_activity: ch.last_published_at || null,
      status: ch.has_published ? 'active' : (ch.enabled ? 'idle' : 'never'),
      enabled: ch.enabled,
    }))
  } catch (err) {
    error.value = 'Failed to load channel data'
    streams.value = []
  } finally {
    loading.value = false
  }
}

const formatDate = (date: string | null) => {
  if (!date) return 'Never'
  const d = new Date(date)
  const diff = Date.now() - d.getTime()
  if (diff < 60000) return 'just now'
  if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`
  if (diff < 86400000) return `${Math.floor(diff / 3600000)}h ago`
  return d.toLocaleDateString()
}

const getStatusVariant = (status: string) => {
  if (status === 'active') return 'success'
  if (status === 'idle') return 'secondary'
  return 'outline'
}

const getStatusLabel = (status: string, enabled: boolean) => {
  if (!enabled) return 'disabled'
  if (status === 'active') return 'active'
  if (status === 'idle') return 'idle'
  return 'never published'
}

onMounted(loadStreams)
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-3xl font-bold tracking-tight">
          Redis Streams
        </h1>
        <p class="text-muted-foreground">
          Redis pub/sub channels and publishing activity
        </p>
      </div>
      <Button
        variant="outline"
        @click="loadStreams"
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

    <Card v-else-if="streams.length === 0">
      <CardContent class="flex flex-col items-center justify-center py-12">
        <Activity class="h-12 w-12 text-muted-foreground mb-4" />
        <h3 class="text-lg font-medium mb-2">
          No active streams
        </h3>
        <p class="text-muted-foreground">
          Redis streams will appear here when channels are active.
        </p>
      </CardContent>
    </Card>

    <div
      v-else
      class="grid gap-4 md:grid-cols-2 lg:grid-cols-3"
    >
      <Card
        v-for="stream in streams"
        :key="stream.name"
      >
        <CardHeader class="pb-2">
          <div class="flex items-center justify-between">
            <CardTitle class="text-base font-mono">
              {{ stream.name }}
            </CardTitle>
            <Badge :variant="getStatusVariant(stream.status)">
              {{ getStatusLabel(stream.status, stream.enabled) }}
            </Badge>
          </div>
        </CardHeader>
        <CardContent>
          <dl class="text-sm">
            <div>
              <dt class="text-muted-foreground">
                Total Published
              </dt>
              <dd class="text-2xl font-bold">
                {{ stream.messages_count.toLocaleString() }}
              </dd>
            </div>
          </dl>
          <p class="mt-4 text-xs text-muted-foreground">
            Last published: {{ formatDate(stream.last_activity) }}
          </p>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
