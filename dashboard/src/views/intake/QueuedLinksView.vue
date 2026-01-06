<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Loader2, Link, Trash2 } from 'lucide-vue-next'
import { crawlerApi } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'

interface QueuedLink {
  id: string
  url: string
  source_name: string
  created_at: string
  status: string
}

const loading = ref(true)
const error = ref<string | null>(null)
const links = ref<QueuedLink[]>([])

const loadLinks = async () => {
  try {
    loading.value = true
    const response = await crawlerApi.queuedLinks.list()
    links.value = response.data?.links || response.data || []
  } catch (err) {
    error.value = 'Unable to load queued links.'
  } finally {
    loading.value = false
  }
}

const deleteLink = async (id: string) => {
  try {
    await crawlerApi.queuedLinks.delete(id)
    links.value = links.value.filter((l) => l.id !== id)
  } catch (err) {
    console.error('Error deleting link:', err)
  }
}

const formatDate = (date: string) => date ? new Date(date).toLocaleString() : 'N/A'

onMounted(loadLinks)
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-3xl font-bold tracking-tight">Queued Links</h1>
      <p class="text-muted-foreground">Links discovered during crawling awaiting processing</p>
    </div>

    <div v-if="loading" class="flex items-center justify-center py-12">
      <Loader2 class="h-8 w-8 animate-spin text-muted-foreground" />
    </div>

    <Card v-else-if="error" class="border-destructive">
      <CardContent class="pt-6">
        <p class="text-destructive">{{ error }}</p>
      </CardContent>
    </Card>

    <Card v-else-if="links.length === 0">
      <CardContent class="flex flex-col items-center justify-center py-12">
        <Link class="h-12 w-12 text-muted-foreground mb-4" />
        <h3 class="text-lg font-medium mb-2">No queued links</h3>
        <p class="text-muted-foreground">Links discovered during crawling will appear here.</p>
      </CardContent>
    </Card>

    <Card v-else>
      <CardContent class="p-0">
        <table class="w-full">
          <thead class="border-b bg-muted/50">
            <tr>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">URL</th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">Source</th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">Created</th>
              <th class="px-6 py-3 text-right text-xs font-medium text-muted-foreground uppercase">Actions</th>
            </tr>
          </thead>
          <tbody class="divide-y">
            <tr v-for="link in links" :key="link.id" class="hover:bg-muted/50">
              <td class="px-6 py-4 text-sm">
                <a :href="link.url" target="_blank" class="text-primary hover:underline truncate block max-w-md">
                  {{ link.url }}
                </a>
              </td>
              <td class="px-6 py-4 text-sm text-muted-foreground">{{ link.source_name }}</td>
              <td class="px-6 py-4 text-sm text-muted-foreground">{{ formatDate(link.created_at) }}</td>
              <td class="px-6 py-4 text-right">
                <Button variant="ghost" size="icon" @click="deleteLink(link.id)">
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
