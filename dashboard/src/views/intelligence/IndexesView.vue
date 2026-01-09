<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Loader2, Database, RefreshCw, Trash2, AlertTriangle } from 'lucide-vue-next'
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

// Delete confirmation state
const deleteModalOpen = ref(false)
const indexToDelete = ref<string | null>(null)
const deleting = ref(false)
const deleteError = ref<string | null>(null)

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

const confirmDelete = (indexName: string) => {
  indexToDelete.value = indexName
  deleteError.value = null
  deleteModalOpen.value = true
}

const cancelDelete = () => {
  deleteModalOpen.value = false
  indexToDelete.value = null
  deleteError.value = null
}

const deleteIndex = async () => {
  if (!indexToDelete.value) return

  try {
    deleting.value = true
    deleteError.value = null
    await indexManagerApi.indexes.delete(indexToDelete.value)
    deleteModalOpen.value = false
    indexToDelete.value = null
    // Reload the list after successful deletion
    await loadIndexes()
  } catch (err) {
    console.error('Failed to delete index:', err)
    deleteError.value = 'Failed to delete index. Please try again.'
  } finally {
    deleting.value = false
  }
}

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
                  title="Delete index"
                  @click.stop="confirmDelete(index.name)"
                >
                  <Trash2 class="h-4 w-4 text-destructive" />
                </Button>
              </td>
            </tr>
          </tbody>
        </table>
      </CardContent>
    </Card>

    <!-- Delete Confirmation Modal -->
    <div
      v-if="deleteModalOpen"
      class="fixed inset-0 z-50 flex items-center justify-center"
    >
      <!-- Backdrop -->
      <div
        class="fixed inset-0 bg-black/50"
        @click="cancelDelete"
      />
      
      <!-- Modal -->
      <Card class="relative z-10 w-full max-w-md mx-4">
        <CardContent class="pt-6">
          <div class="flex items-start gap-4">
            <div class="flex-shrink-0 w-10 h-10 rounded-full bg-destructive/10 flex items-center justify-center">
              <AlertTriangle class="h-5 w-5 text-destructive" />
            </div>
            <div class="flex-1">
              <h3 class="text-lg font-semibold mb-2">
                Delete Index
              </h3>
              <p class="text-sm text-muted-foreground mb-1">
                Are you sure you want to delete the index:
              </p>
              <p class="text-sm font-mono font-medium mb-4 break-all">
                {{ indexToDelete }}
              </p>
              <p class="text-sm text-destructive mb-4">
                This action cannot be undone. All documents in this index will be permanently deleted.
              </p>

              <div
                v-if="deleteError"
                class="text-sm text-destructive bg-destructive/10 px-3 py-2 rounded mb-4"
              >
                {{ deleteError }}
              </div>

              <div class="flex justify-end gap-2">
                <Button
                  variant="outline"
                  :disabled="deleting"
                  @click="cancelDelete"
                >
                  Cancel
                </Button>
                <Button
                  variant="destructive"
                  :disabled="deleting"
                  @click="deleteIndex"
                >
                  <Loader2
                    v-if="deleting"
                    class="mr-2 h-4 w-4 animate-spin"
                  />
                  {{ deleting ? 'Deleting...' : 'Delete Index' }}
                </Button>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
