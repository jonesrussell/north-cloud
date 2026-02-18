<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Loader2, Radio, Plus, Pencil, Trash2 } from 'lucide-vue-next'
import { publisherApi } from '@/api/client'
import type { TopicInfo } from '@/types/publisher'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'

interface Channel {
  id: number
  name: string
  description: string
  enabled: boolean
  routes_count: number
}

const loading = ref(true)
const error = ref<string | null>(null)
const channels = ref<Channel[]>([])

const topicChannels = ref<TopicInfo[]>([])
const topicsLoading = ref(true)
const topicsError = ref<string | null>(null)

const loadChannels = async () => {
  try {
    loading.value = true
    const response = await publisherApi.channels.list()
    channels.value = response.data?.channels || []
  } catch {
    error.value = 'Unable to load channels.'
  } finally {
    loading.value = false
  }
}

const loadTopics = async () => {
  try {
    topicsLoading.value = true
    topicsError.value = null
    const response = await publisherApi.topics.list()
    topicChannels.value = response.data?.topics ?? []
  } catch {
    topicsError.value = 'Could not load topic channels.'
  } finally {
    topicsLoading.value = false
  }
}

const deleteChannel = async (id: number) => {
  if (!confirm('Delete this channel?')) return
  try {
    await publisherApi.channels.delete(id)
    channels.value = channels.value.filter((c) => c.id !== id)
  } catch (err) {
    console.error('Error deleting channel:', err)
  }
}

onMounted(() => {
  loadChannels()
  loadTopics()
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-3xl font-bold tracking-tight">
          Channels
        </h1>
        <p class="text-muted-foreground">
          Channels define what content is published. Any number of consumers may subscribe to each channel.
        </p>
      </div>
      <Button>
        <Plus class="mr-2 h-4 w-4" />
        New Channel
      </Button>
    </div>

    <!-- Topic channels (Layer 1 - automatic) -->
    <Card>
      <CardContent class="pt-6">
        <h2 class="text-lg font-semibold mb-1">
          Topic channels (automatic)
        </h2>
        <p class="text-sm text-muted-foreground mb-4">
          Articles are published to these Redis channels by topic. No configuration needed.
        </p>
        <div
          v-if="topicsLoading"
          class="flex items-center justify-center py-8 text-muted-foreground"
        >
          <Loader2 class="h-6 w-6 animate-spin" />
        </div>
        <p
          v-else-if="topicsError"
          class="text-sm text-muted-foreground py-4"
        >
          {{ topicsError }}
        </p>
        <div
          v-else-if="topicChannels.length > 0"
          class="rounded-md border overflow-x-auto"
        >
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b bg-muted/50">
                <th
                  class="h-9 px-4 text-left font-medium"
                  scope="col"
                >
                  Topic
                </th>
                <th
                  class="h-9 px-4 text-left font-medium"
                  scope="col"
                >
                  Redis channel
                </th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="topic in topicChannels"
                :key="topic.name"
                class="border-b last:border-0"
              >
                <td class="px-4 py-2">
                  {{ topic.name }}
                </td>
                <td class="px-4 py-2 font-mono text-muted-foreground">
                  {{ topic.layer1_channel }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </CardContent>
    </Card>

    <!-- Custom channels (Layer 2) -->
    <h2 class="text-lg font-semibold">
      Custom channels
    </h2>
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

    <Card v-else-if="channels.length === 0">
      <CardContent class="flex flex-col items-center justify-center py-12">
        <Radio class="h-12 w-12 text-muted-foreground mb-4" />
        <h3 class="text-lg font-medium mb-2">
          No channels configured
        </h3>
        <p class="text-muted-foreground mb-4">
          Channels are content streams (by topic or custom rules). The publisher does not track who subscribes. Add custom channels below when you need aggregations or specific filters.
        </p>
        <Button>
          <Plus class="mr-2 h-4 w-4" />
          New Channel
        </Button>
      </CardContent>
    </Card>

    <div
      v-else
      class="grid gap-4 md:grid-cols-2 lg:grid-cols-3"
    >
      <Card
        v-for="channel in channels"
        :key="channel.id"
        class="hover:shadow-md transition-shadow"
      >
        <CardContent class="pt-6">
          <div class="flex items-start justify-between mb-4">
            <div>
              <h3 class="font-semibold">
                {{ channel.name }}
              </h3>
              <p class="text-sm text-muted-foreground mt-1">
                {{ channel.description || 'No description' }}
              </p>
            </div>
            <Badge :variant="channel.enabled ? 'success' : 'secondary'">
              {{ channel.enabled ? 'Active' : 'Inactive' }}
            </Badge>
          </div>
          <div class="flex items-center justify-between text-sm text-muted-foreground">
            <span>{{ channel.routes_count || 0 }} routes</span>
            <div class="flex gap-1">
              <Button
                variant="ghost"
                size="icon"
                class="h-8 w-8"
              >
                <Pencil class="h-4 w-4" />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                class="h-8 w-8"
                @click="deleteChannel(channel.id)"
              >
                <Trash2 class="h-4 w-4 text-destructive" />
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
