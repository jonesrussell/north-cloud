<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import {
  Loader2,
  GitBranch,
  RefreshCw,
  Radio,
  Zap,
  Settings,
  ArrowRight,
} from 'lucide-vue-next'
import { publisherApi } from '@/api/client'
import type { TopicInfo } from '@/types/publisher'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'

const router = useRouter()

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
    const response = await publisherApi.topics.list()
    topicChannels.value = response.data?.topics ?? []
  } catch (err) {
    console.error('Failed to load topics:', err)
  } finally {
    topicsLoading.value = false
  }
}

const refresh = async () => {
  await Promise.all([loadChannels(), loadTopics()])
}

const goToChannels = () => {
  router.push('/distribution/channels')
}

const getCrimeColor = (topic: string) => {
  if (topic.includes('violent')) return 'bg-red-500'
  if (topic.includes('property')) return 'bg-orange-500'
  if (topic.includes('drug')) return 'bg-purple-500'
  if (topic.includes('organized')) return 'bg-yellow-600'
  if (topic.includes('justice')) return 'bg-blue-500'
  return 'bg-gray-500'
}

onMounted(refresh)
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-3xl font-bold tracking-tight">
          Routes
        </h1>
        <p class="text-muted-foreground">
          Content routing from indexes to Redis pub/sub channels
        </p>
      </div>
      <Button
        variant="outline"
        :disabled="loading || topicsLoading"
        @click="refresh"
      >
        <RefreshCw
          class="mr-2 h-4 w-4"
          :class="{ 'animate-spin': loading || topicsLoading }"
        />
        Refresh
      </Button>
    </div>

    <div
      v-if="loading && topicsLoading"
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

    <template v-else>
      <!-- Layer 1: Automatic Topic Routes -->
      <Card>
        <CardHeader>
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <Zap class="h-5 w-5 text-yellow-500" />
              <CardTitle>Automatic Topic Routes</CardTitle>
            </div>
            <Badge variant="secondary">
              Layer 1
            </Badge>
          </div>
          <CardDescription>
            Articles automatically route to <code>articles:{topic}</code> channels based on their topics.
            No configuration needed.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div
            v-if="topicsLoading"
            class="flex items-center justify-center py-8"
          >
            <Loader2 class="h-6 w-6 animate-spin text-muted-foreground" />
          </div>
          <div
            v-else-if="topicChannels.length === 0"
            class="text-center py-8 text-muted-foreground"
          >
            No topic channels active yet. Articles will create channels when published.
          </div>
          <div
            v-else
            class="space-y-4"
          >
            <!-- Crime Topics -->
            <div
              v-if="topicChannels.some(t => t.topic.includes('crime'))"
              class="space-y-2"
            >
              <h4 class="text-sm font-medium text-muted-foreground">
                Crime Topics
              </h4>
              <div class="flex flex-wrap gap-2">
                <div
                  v-for="topic in topicChannels.filter(t => t.topic.includes('crime'))"
                  :key="topic.topic"
                  class="flex items-center gap-2 px-3 py-2 bg-muted rounded-md"
                >
                  <div
                    class="w-2 h-2 rounded-full"
                    :class="getCrimeColor(topic.topic)"
                  />
                  <code class="text-sm">articles:{{ topic.topic }}</code>
                  <ArrowRight class="h-3 w-3 text-muted-foreground" />
                  <Badge variant="outline">
                    {{ topic.subscriber_count }} subscribers
                  </Badge>
                </div>
              </div>
            </div>

            <!-- Other Topics -->
            <div
              v-if="topicChannels.some(t => !t.topic.includes('crime'))"
              class="space-y-2"
            >
              <h4 class="text-sm font-medium text-muted-foreground">
                Other Topics
              </h4>
              <div class="flex flex-wrap gap-2">
                <div
                  v-for="topic in topicChannels.filter(t => !t.topic.includes('crime'))"
                  :key="topic.topic"
                  class="flex items-center gap-2 px-3 py-2 bg-muted rounded-md"
                >
                  <Radio class="h-3 w-3 text-muted-foreground" />
                  <code class="text-sm">articles:{{ topic.topic }}</code>
                  <ArrowRight class="h-3 w-3 text-muted-foreground" />
                  <Badge variant="outline">
                    {{ topic.subscriber_count }} subscribers
                  </Badge>
                </div>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      <!-- Layer 2: Custom Channel Routes -->
      <Card>
        <CardHeader>
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <Settings class="h-5 w-5 text-blue-500" />
              <CardTitle>Custom Channel Routes</CardTitle>
            </div>
            <Badge variant="secondary">
              Layer 2
            </Badge>
          </div>
          <CardDescription>
            Custom channels with filtering rules (quality score, topics, content type).
            Managed in the Channels view.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div
            v-if="loading"
            class="flex items-center justify-center py-8"
          >
            <Loader2 class="h-6 w-6 animate-spin text-muted-foreground" />
          </div>
          <div
            v-else-if="channels.length === 0"
            class="text-center py-8"
          >
            <GitBranch class="h-12 w-12 text-muted-foreground mx-auto mb-4" />
            <p class="text-muted-foreground mb-4">
              No custom channels configured yet.
            </p>
            <Button
              variant="outline"
              @click="goToChannels"
            >
              Create Channel
            </Button>
          </div>
          <div
            v-else
            class="space-y-3"
          >
            <div
              v-for="channel in channels"
              :key="channel.id"
              class="flex items-center justify-between p-3 bg-muted/50 rounded-md"
            >
              <div class="flex items-center gap-3">
                <div
                  class="w-2 h-2 rounded-full"
                  :class="channel.enabled ? 'bg-green-500' : 'bg-gray-400'"
                />
                <div>
                  <p class="font-medium">
                    {{ channel.name }}
                  </p>
                  <p class="text-sm text-muted-foreground">
                    {{ channel.description || 'No description' }}
                  </p>
                </div>
              </div>
              <div class="flex items-center gap-2">
                <Badge :variant="channel.enabled ? 'default' : 'secondary'">
                  {{ channel.enabled ? 'Active' : 'Disabled' }}
                </Badge>
                <Badge variant="outline">
                  {{ channel.routes_count }} route{{ channel.routes_count !== 1 ? 's' : '' }}
                </Badge>
              </div>
            </div>
            <div class="pt-2">
              <Button
                variant="outline"
                size="sm"
                @click="goToChannels"
              >
                Manage Channels
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      <!-- Layer 3: Crime Classification Routes -->
      <Card>
        <CardHeader>
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <Zap class="h-5 w-5 text-red-500" />
              <CardTitle>Crime Classification Routes</CardTitle>
            </div>
            <Badge variant="secondary">
              Layer 3
            </Badge>
          </div>
          <CardDescription>
            Automatic routing based on classifier's crime detection.
            Homepage-eligible articles route to <code>crime:homepage</code>,
            category listings to <code>crime:category:{type}</code>.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div class="space-y-3">
            <div class="flex items-center gap-2 px-3 py-2 bg-muted rounded-md">
              <div class="w-2 h-2 rounded-full bg-red-500" />
              <code class="text-sm">crime:homepage</code>
              <ArrowRight class="h-3 w-3 text-muted-foreground" />
              <span class="text-sm text-muted-foreground">High-confidence crime articles for homepage</span>
            </div>
            <div class="flex items-center gap-2 px-3 py-2 bg-muted rounded-md">
              <div class="w-2 h-2 rounded-full bg-orange-500" />
              <code class="text-sm">crime:category:{"{type}"}</code>
              <ArrowRight class="h-3 w-3 text-muted-foreground" />
              <span class="text-sm text-muted-foreground">Category page listings by crime type</span>
            </div>
          </div>
        </CardContent>
      </Card>
    </template>
  </div>
</template>
