<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { formatDate } from '@/lib/utils'
import { Loader2, Link, Trash2, RefreshCw } from 'lucide-vue-next'
import { crawlerApi } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'

// Match the actual API response from crawler service
interface DiscoveredLink {
  id: string
  source_id: string
  source_name: string
  url: string
  parent_url: string | null
  depth: number
  discovered_at: string
  status: string
  priority: number
}

const loading = ref(true)
const error = ref<string | null>(null)
const links = ref<DiscoveredLink[]>([])
const total = ref(0)

const loadLinks = async () => {
  try {
    loading.value = true
    error.value = null
    const response = await crawlerApi.discoveredLinks.list()
    links.value = response.data?.links || []
    total.value = response.data?.total || links.value.length
  } catch (err) {
    console.error('Failed to load discovered links:', err)
    error.value = 'Unable to load discovered links.'
  } finally {
    loading.value = false
  }
}

const deleteLink = async (id: string) => {
  if (!confirm('Delete this discovered link?')) return
  try {
    await crawlerApi.discoveredLinks.delete(id)
    links.value = links.value.filter((l) => l.id !== id)
  } catch (err) {
    console.error('Error deleting link:', err)
  }
}

const getStatusVariant = (status: string) => {
  switch (status) {
    case 'pending': return 'secondary'
    case 'processing': return 'warning'
    case 'completed': return 'success'
    case 'failed': return 'destructive'
    default: return 'outline'
  }
}

onMounted(loadLinks)
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-3xl font-bold tracking-tight">
          Discovered Links
        </h1>
        <p class="text-muted-foreground">
          Links discovered during crawling awaiting processing
        </p>
      </div>
      <Button
        variant="outline"
        @click="loadLinks"
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

    <Card v-else-if="links.length === 0">
      <CardContent class="flex flex-col items-center justify-center py-12">
        <Link class="h-12 w-12 text-muted-foreground mb-4" />
        <h3 class="text-lg font-medium mb-2">
          No discovered links
        </h3>
        <p class="text-muted-foreground">
          Links discovered during crawling will appear here.
        </p>
      </CardContent>
    </Card>

    <Card v-else>
      <CardContent class="p-0">
        <table class="w-full">
          <thead class="border-b bg-muted/50">
            <tr>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                URL
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Source
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Depth
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Status
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Discovered
              </th>
              <th class="px-6 py-3 text-right text-xs font-medium text-muted-foreground uppercase">
                Actions
              </th>
            </tr>
          </thead>
          <tbody class="divide-y">
            <tr
              v-for="link in links"
              :key="link.id"
              class="hover:bg-muted/50"
            >
              <td class="px-6 py-4 text-sm">
                <a
                  :href="link.url"
                  target="_blank"
                  class="text-primary hover:underline truncate block max-w-md"
                >
                  {{ link.url }}
                </a>
              </td>
              <td class="px-6 py-4 text-sm text-muted-foreground">
                {{ link.source_name }}
              </td>
              <td class="px-6 py-4 text-sm text-muted-foreground">
                {{ link.depth }}
              </td>
              <td class="px-6 py-4">
                <Badge :variant="getStatusVariant(link.status)">
                  {{ link.status }}
                </Badge>
              </td>
              <td class="px-6 py-4 text-sm text-muted-foreground">
                {{ formatDate(link.discovered_at) }}
              </td>
              <td class="px-6 py-4 text-right">
                <Button
                  variant="ghost"
                  size="icon"
                  @click="deleteLink(link.id)"
                >
                  <Trash2 class="h-4 w-4 text-destructive" />
                </Button>
              </td>
            </tr>
          </tbody>
        </table>
      </CardContent>
    </Card>
  </div>
</template>
