<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Loader2, GitBranch, Plus, Pencil, Trash2 } from 'lucide-vue-next'
import { publisherApi } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'

interface Route {
  id: number
  source_name: string
  channel_name: string
  min_quality_score: number
  topics: string[]
  enabled: boolean
}

const loading = ref(true)
const error = ref<string | null>(null)
const routes = ref<Route[]>([])

const loadRoutes = async () => {
  try {
    loading.value = true
    const response = await publisherApi.routes.list()
    routes.value = response.data?.routes || []
  } catch (err) {
    error.value = 'Unable to load routes.'
  } finally {
    loading.value = false
  }
}

const deleteRoute = async (id: number) => {
  if (!confirm('Delete this route?')) return
  try {
    await publisherApi.routes.delete(id)
    routes.value = routes.value.filter((r) => r.id !== id)
  } catch (err) {
    console.error('Error deleting route:', err)
  }
}

onMounted(loadRoutes)
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-3xl font-bold tracking-tight">Routes</h1>
        <p class="text-muted-foreground">Configure how content flows to channels</p>
      </div>
      <Button>
        <Plus class="mr-2 h-4 w-4" />
        New Route
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

    <Card v-else-if="routes.length === 0">
      <CardContent class="flex flex-col items-center justify-center py-12">
        <GitBranch class="h-12 w-12 text-muted-foreground mb-4" />
        <h3 class="text-lg font-medium mb-2">No routes configured</h3>
        <p class="text-muted-foreground mb-4">Create routes to publish content to channels.</p>
        <Button>
          <Plus class="mr-2 h-4 w-4" />
          New Route
        </Button>
      </CardContent>
    </Card>

    <Card v-else>
      <CardContent class="p-0">
        <table class="w-full">
          <thead class="border-b bg-muted/50">
            <tr>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">Source</th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">Channel</th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">Min Quality</th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">Topics</th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">Status</th>
              <th class="px-6 py-3 text-right text-xs font-medium text-muted-foreground uppercase">Actions</th>
            </tr>
          </thead>
          <tbody class="divide-y">
            <tr v-for="route in routes" :key="route.id" class="hover:bg-muted/50">
              <td class="px-6 py-4 text-sm font-medium">{{ route.source_name }}</td>
              <td class="px-6 py-4 text-sm text-primary">{{ route.channel_name }}</td>
              <td class="px-6 py-4 text-sm text-muted-foreground">{{ route.min_quality_score }}/100</td>
              <td class="px-6 py-4">
                <div class="flex gap-1 flex-wrap">
                  <Badge v-for="topic in route.topics?.slice(0, 3)" :key="topic" variant="outline" class="text-xs">
                    {{ topic }}
                  </Badge>
                  <Badge v-if="(route.topics?.length || 0) > 3" variant="outline" class="text-xs">
                    +{{ route.topics.length - 3 }}
                  </Badge>
                </div>
              </td>
              <td class="px-6 py-4">
                <Badge :variant="route.enabled ? 'success' : 'secondary'">
                  {{ route.enabled ? 'Active' : 'Inactive' }}
                </Badge>
              </td>
              <td class="px-6 py-4 text-right">
                <div class="flex justify-end gap-2">
                  <Button variant="ghost" size="icon">
                    <Pencil class="h-4 w-4" />
                  </Button>
                  <Button variant="ghost" size="icon" @click="deleteRoute(route.id)">
                    <Trash2 class="h-4 w-4 text-destructive" />
                  </Button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </CardContent>
    </Card>
  </div>
</template>
