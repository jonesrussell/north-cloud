<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Loader2, Database, RefreshCw, Trash2 } from 'lucide-vue-next'
import { indexManagerApi } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import type { Index } from '@/types/indexManager'

interface DisplayIndex {
  name: string
  document_count: number
  size: string
  health: string
  type: string
}

const router = useRouter()
const loading = ref(true)
const error = ref<string | null>(null)
const indexes = ref<DisplayIndex[]>([])

const loadIndexes = async () => {
  try {
    loading.value = true
    error.value = null
    const response = await indexManagerApi.indexes.list()
    // Backend returns 'indices', map to display format
    const rawIndices: Index[] = response.data?.indices || []
    indexes.value = rawIndices.map((idx) => ({
      name: idx.name,
      document_count: idx.document_count || 0,
      size: idx.size || '0 B',
      health: idx.health || 'unknown',
      type: idx.type || 'unknown',
    }))
  } catch (err) {
    console.error('Failed to load indexes:', err)
    error.value = 'Unable to load Elasticsearch indexes.'
  } finally {
    loading.value = false
  }
}

const getHealthVariant = (health: string) => {
  switch (health) {
    case 'green': return 'success'
    case 'yellow': return 'warning'
    case 'red': return 'destructive'
    default: return 'secondary'
  }
}

const viewIndex = (name: string) => router.push(`/intelligence/indexes/${name}`)

onMounted(loadIndexes)
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-3xl font-bold tracking-tight">
          Elasticsearch Indexes
        </h1>
        <p class="text-muted-foreground">
          Manage content indexes and documents
        </p>
      </div>
      <Button
        variant="outline"
        @click="loadIndexes"
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

    <Card v-else-if="indexes.length === 0">
      <CardContent class="flex flex-col items-center justify-center py-12">
        <Database class="h-12 w-12 text-muted-foreground mb-4" />
        <h3 class="text-lg font-medium mb-2">
          No indexes found
        </h3>
        <p class="text-muted-foreground">
          Indexes will be created automatically when content is crawled.
        </p>
      </CardContent>
    </Card>

    <Card v-else>
      <CardContent class="p-0">
        <table class="w-full">
          <thead class="border-b bg-muted/50">
            <tr>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Name
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Type
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Documents
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Size
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Health
              </th>
              <th class="px-6 py-3 text-right text-xs font-medium text-muted-foreground uppercase">
                Actions
              </th>
            </tr>
          </thead>
          <tbody class="divide-y">
            <tr 
              v-for="index in indexes" 
              :key="index.name" 
              class="hover:bg-muted/50 cursor-pointer"
              @click="viewIndex(index.name)"
            >
              <td class="px-6 py-4 text-sm font-medium">
                <button
                  class="text-primary hover:underline"
                  @click.stop="viewIndex(index.name)"
                >
                  {{ index.name }}
                </button>
              </td>
              <td class="px-6 py-4 text-sm text-muted-foreground">
                {{ index.type || 'content' }}
              </td>
              <td class="px-6 py-4 text-sm text-muted-foreground">
                {{ index.document_count.toLocaleString() }}
              </td>
              <td class="px-6 py-4 text-sm text-muted-foreground">
                {{ index.size }}
              </td>
              <td class="px-6 py-4">
                <Badge :variant="getHealthVariant(index.health)">
                  {{ index.health }}
                </Badge>
              </td>
              <td class="px-6 py-4 text-right">
                <Button
                  variant="ghost"
                  size="icon"
                  @click.stop
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
