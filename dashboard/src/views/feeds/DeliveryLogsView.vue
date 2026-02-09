<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { formatDate } from '@/lib/utils'
import { Loader2, ScrollText, RefreshCw, CheckCircle2, XCircle, Clock } from 'lucide-vue-next'
import { publisherApi } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'

interface DeliveryLog {
  id: string
  article_id: string
  article_title: string
  channel_name: string
  quality_score: number
  status: 'delivered' | 'failed' | 'pending'
  published_at: string
}

const loading = ref(true)
const error = ref<string | null>(null)
const logs = ref<DeliveryLog[]>([])

const loadLogs = async () => {
  try {
    loading.value = true
    const response = await publisherApi.history.list({ limit: 50 })
    logs.value = (response.data?.history || []).map((h: Record<string, unknown>) => ({
      id: h.id as string || String(Math.random()),
      article_id: h.article_id as string || '',
      article_title: h.article_title as string || 'Untitled',
      channel_name: h.channel_name as string || '',
      quality_score: h.quality_score as number || 0,
      status: 'delivered',
      published_at: h.published_at as string || new Date().toISOString(),
    }))
  } catch (err) {
    error.value = 'Unable to load delivery logs.'
  } finally {
    loading.value = false
  }
}

const getStatusIcon = (status: string) => {
  switch (status) {
    case 'delivered': return CheckCircle2
    case 'failed': return XCircle
    default: return Clock
  }
}

const getStatusVariant = (status: string) => {
  switch (status) {
    case 'delivered': return 'success'
    case 'failed': return 'destructive'
    default: return 'warning'
  }
}

onMounted(loadLogs)
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-3xl font-bold tracking-tight">
          Delivery Logs
        </h1>
        <p class="text-muted-foreground">
          Track article publication to channels
        </p>
      </div>
      <Button
        variant="outline"
        @click="loadLogs"
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

    <Card v-else-if="logs.length === 0">
      <CardContent class="flex flex-col items-center justify-center py-12">
        <ScrollText class="h-12 w-12 text-muted-foreground mb-4" />
        <h3 class="text-lg font-medium mb-2">
          No delivery logs
        </h3>
        <p class="text-muted-foreground">
          Logs will appear here when articles are published.
        </p>
      </CardContent>
    </Card>

    <Card v-else>
      <CardHeader>
        <CardTitle>Recent Deliveries</CardTitle>
        <CardDescription>Showing the {{ logs.length }} most recent publications</CardDescription>
      </CardHeader>
      <CardContent class="p-0">
        <table class="w-full">
          <thead class="border-b bg-muted/50">
            <tr>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Status
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Article
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Channel
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Quality
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Time
              </th>
            </tr>
          </thead>
          <tbody class="divide-y">
            <tr
              v-for="log in logs"
              :key="log.id"
              class="hover:bg-muted/50"
            >
              <td class="px-6 py-4">
                <div class="flex items-center gap-2">
                  <component
                    :is="getStatusIcon(log.status)" 
                    :class="['h-4 w-4', log.status === 'delivered' ? 'text-green-500' : log.status === 'failed' ? 'text-red-500' : 'text-yellow-500']" 
                  />
                  <Badge :variant="getStatusVariant(log.status)">
                    {{ log.status }}
                  </Badge>
                </div>
              </td>
              <td class="px-6 py-4 text-sm">
                <p class="truncate max-w-xs font-medium">
                  {{ log.article_title }}
                </p>
                <p class="text-xs text-muted-foreground font-mono">
                  {{ log.article_id }}
                </p>
              </td>
              <td class="px-6 py-4">
                <Badge variant="outline">
                  {{ log.channel_name }}
                </Badge>
              </td>
              <td class="px-6 py-4 text-sm text-muted-foreground">
                {{ log.quality_score }}/100
              </td>
              <td class="px-6 py-4 text-sm text-muted-foreground">
                {{ formatDate(log.published_at) }}
              </td>
            </tr>
          </tbody>
        </table>
      </CardContent>
    </Card>
  </div>
</template>
