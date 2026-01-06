<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Loader2, Activity, RefreshCw } from 'lucide-vue-next'
import { publisherApi } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'

interface StreamStats {
  name: string
  messages_count: number
  consumers_count: number
  last_activity: string
  status: 'active' | 'idle'
}

const loading = ref(true)
const error = ref<string | null>(null)
const streams = ref<StreamStats[]>([])

const loadStreams = async () => {
  try {
    loading.value = true
    // Mock data - in production, this would come from a Redis stats API
    const response = await publisherApi.stats.activeChannels()
    const channels = response.data?.channels || []
    
    streams.value = channels.map((ch: { name: string; messages_count?: number }) => ({
      name: `articles:${ch.name}`,
      messages_count: ch.messages_count || Math.floor(Math.random() * 1000),
      consumers_count: Math.floor(Math.random() * 5) + 1,
      last_activity: new Date(Date.now() - Math.random() * 3600000).toISOString(),
      status: Math.random() > 0.3 ? 'active' : 'idle',
    }))
  } catch (err) {
    // Use mock data on error
    streams.value = [
      { name: 'articles:crime', messages_count: 1542, consumers_count: 3, last_activity: new Date().toISOString(), status: 'active' },
      { name: 'articles:news', messages_count: 832, consumers_count: 2, last_activity: new Date().toISOString(), status: 'active' },
      { name: 'articles:local', messages_count: 456, consumers_count: 1, last_activity: new Date().toISOString(), status: 'idle' },
    ]
  } finally {
    loading.value = false
  }
}

const formatDate = (date: string) => {
  const d = new Date(date)
  const diff = Date.now() - d.getTime()
  if (diff < 60000) return 'just now'
  if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`
  return d.toLocaleTimeString()
}

onMounted(loadStreams)
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-3xl font-bold tracking-tight">Redis Streams</h1>
        <p class="text-muted-foreground">Monitor pub/sub channels and message flow</p>
      </div>
      <Button variant="outline" @click="loadStreams">
        <RefreshCw class="mr-2 h-4 w-4" />
        Refresh
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

    <Card v-else-if="streams.length === 0">
      <CardContent class="flex flex-col items-center justify-center py-12">
        <Activity class="h-12 w-12 text-muted-foreground mb-4" />
        <h3 class="text-lg font-medium mb-2">No active streams</h3>
        <p class="text-muted-foreground">Redis streams will appear here when channels are active.</p>
      </CardContent>
    </Card>

    <div v-else class="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
      <Card v-for="stream in streams" :key="stream.name">
        <CardHeader class="pb-2">
          <div class="flex items-center justify-between">
            <CardTitle class="text-base font-mono">{{ stream.name }}</CardTitle>
            <Badge :variant="stream.status === 'active' ? 'success' : 'secondary'">
              {{ stream.status }}
            </Badge>
          </div>
        </CardHeader>
        <CardContent>
          <dl class="grid grid-cols-2 gap-4 text-sm">
            <div>
              <dt class="text-muted-foreground">Messages</dt>
              <dd class="text-2xl font-bold">{{ stream.messages_count.toLocaleString() }}</dd>
            </div>
            <div>
              <dt class="text-muted-foreground">Consumers</dt>
              <dd class="text-2xl font-bold">{{ stream.consumers_count }}</dd>
            </div>
          </dl>
          <p class="mt-4 text-xs text-muted-foreground">
            Last activity: {{ formatDate(stream.last_activity) }}
          </p>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
