<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { formatDateShort } from '@/lib/utils'
import { Loader2, Globe, Plus, Pencil, Trash2, Upload } from 'lucide-vue-next'
import { sourcesApi } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { ImportExcelModal } from '@/components/common'

interface Source {
  id: string
  name: string
  url: string
  enabled: boolean
  created_at: string
}

const router = useRouter()
const loading = ref(true)
const error = ref<string | null>(null)
const sources = ref<Source[]>([])
const deleting = ref<string | null>(null)
const importExcelModalRef = ref<InstanceType<typeof ImportExcelModal> | null>(null)

const loadSources = async () => {
  try {
    loading.value = true
    const response = await sourcesApi.list()
    sources.value = response.data?.sources || response.data || []
  } catch (err) {
    error.value = 'Unable to load sources.'
  } finally {
    loading.value = false
  }
}

const editSource = (id: string) => router.push(`/scheduling/sources/${id}/edit`)

const deleteSource = async (id: string) => {
  if (!confirm('Are you sure you want to delete this source?')) return
  try {
    deleting.value = id
    await sourcesApi.delete(id)
    sources.value = sources.value.filter((s) => s.id !== id)
  } catch (err) {
    console.error('Error deleting source:', err)
  } finally {
    deleting.value = null
  }
}

const openImportExcel = () => {
  importExcelModalRef.value?.open()
}

const onSourcesImported = () => {
  loadSources()
}

onMounted(loadSources)
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-3xl font-bold tracking-tight">
          Sources
        </h1>
        <p class="text-muted-foreground">
          Manage content sources for crawling
        </p>
      </div>
      <div class="flex gap-2">
        <Button
          variant="outline"
          @click="openImportExcel"
        >
          <Upload class="mr-2 h-4 w-4" />
          Import Excel
        </Button>
        <Button @click="router.push('/scheduling/sources/new')">
          <Plus class="mr-2 h-4 w-4" />
          Add Source
        </Button>
      </div>
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

    <Card v-else-if="sources.length === 0">
      <CardContent class="flex flex-col items-center justify-center py-12">
        <Globe class="h-12 w-12 text-muted-foreground mb-4" />
        <h3 class="text-lg font-medium mb-2">
          No sources configured
        </h3>
        <p class="text-muted-foreground mb-4">
          Add your first source to start crawling content.
        </p>
        <Button @click="router.push('/scheduling/sources/new')">
          <Plus class="mr-2 h-4 w-4" />
          Add Source
        </Button>
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
                URL
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Status
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Created
              </th>
              <th class="px-6 py-3 text-right text-xs font-medium text-muted-foreground uppercase">
                Actions
              </th>
            </tr>
          </thead>
          <tbody class="divide-y">
            <tr
              v-for="source in sources"
              :key="source.id"
              class="hover:bg-muted/50"
            >
              <td class="px-6 py-4 text-sm font-medium">
                {{ source.name }}
              </td>
              <td class="px-6 py-4 text-sm">
                <a
                  :href="source.url"
                  target="_blank"
                  class="text-primary hover:underline truncate block max-w-xs"
                >
                  {{ source.url }}
                </a>
              </td>
              <td class="px-6 py-4">
                <Badge :variant="source.enabled ? 'success' : 'secondary'">
                  {{ source.enabled ? 'Active' : 'Inactive' }}
                </Badge>
              </td>
              <td class="px-6 py-4 text-sm text-muted-foreground">
                {{ formatDateShort(source.created_at) }}
              </td>
              <td class="px-6 py-4 text-right">
                <div class="flex justify-end gap-2">
                  <Button
                    variant="ghost"
                    size="icon"
                    @click="editSource(source.id)"
                  >
                    <Pencil class="h-4 w-4" />
                  </Button>
                  <Button 
                    variant="ghost" 
                    size="icon" 
                    :disabled="deleting === source.id"
                    @click="deleteSource(source.id)"
                  >
                    <Loader2
                      v-if="deleting === source.id"
                      class="h-4 w-4 animate-spin"
                    />
                    <Trash2
                      v-else
                      class="h-4 w-4 text-destructive"
                    />
                  </Button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </CardContent>
    </Card>

    <!-- Import Excel Modal -->
    <ImportExcelModal
      ref="importExcelModalRef"
      @imported="onSourcesImported"
    />
  </div>
</template>
