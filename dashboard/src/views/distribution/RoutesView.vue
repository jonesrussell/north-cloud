<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Loader2, GitBranch, Plus, Pencil, Trash2, X } from 'lucide-vue-next'
import { publisherApi } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import type {
  Route,
  Source,
  Channel,
  CreateRouteRequest,
  UpdateRouteRequest,
} from '@/types/publisher'

const loading = ref(true)
const error = ref<string | null>(null)
const routes = ref<Route[]>([])
const sources = ref<Source[]>([])
const channels = ref<Channel[]>([])

// Modal state
const showModal = ref(false)
const isEditing = ref(false)
const modalError = ref<string | null>(null)
const saving = ref(false)
const currentRoute = ref<Route | null>(null)
const topicsInput = ref('')

// Form data
const formData = ref<CreateRouteRequest>({
  source_id: 0,
  channel_id: 0,
  min_quality_score: 50,
  topics: null,
  enabled: true,
})

const loadRoutes = async () => {
  try {
    loading.value = true
    error.value = null
    const response = await publisherApi.routes.list()
    routes.value = response.data?.routes || []
  } catch (err) {
    error.value = 'Unable to load routes.'
  } finally {
    loading.value = false
  }
}

const loadSources = async () => {
  try {
    const response = await publisherApi.sources.list(true) // Only enabled sources
    sources.value = response.data?.sources || []
  } catch (err) {
    console.error('Failed to load sources:', err)
  }
}

const loadChannels = async () => {
  try {
    const response = await publisherApi.channels.list(false) // All channels
    channels.value = response.data?.channels || []
  } catch (err) {
    console.error('Failed to load channels:', err)
  }
}

const openCreateModal = () => {
  isEditing.value = false
  formData.value = {
    source_id: 0,
    channel_id: 0,
    min_quality_score: 50,
    topics: null,
    enabled: true,
  }
  topicsInput.value = ''
  currentRoute.value = null
  modalError.value = null
  showModal.value = true
}

const openEditModal = (route: Route) => {
  isEditing.value = true
  formData.value = {
    source_id: route.source_id,
    channel_id: route.channel_id,
    min_quality_score: route.min_quality_score,
    topics: route.topics,
    enabled: route.enabled,
  }
  topicsInput.value = (route.topics || []).join(', ')
  currentRoute.value = route
  modalError.value = null
  showModal.value = true
}

const closeModal = () => {
  showModal.value = false
  formData.value = {
    source_id: 0,
    channel_id: 0,
    min_quality_score: 50,
    topics: null,
    enabled: true,
  }
  topicsInput.value = ''
  currentRoute.value = null
  modalError.value = null
}

const saveRoute = async () => {
  saving.value = true
  modalError.value = null

  // Parse topics from comma-separated input
  const topics = topicsInput.value
    .split(',')
    .map((t) => t.trim())
    .filter((t) => t.length > 0)

  const payload: CreateRouteRequest | UpdateRouteRequest = {
    ...formData.value,
    topics: topics.length > 0 ? topics : null,
  }

  try {
    if (isEditing.value && currentRoute.value) {
      await publisherApi.routes.update(
        currentRoute.value.id,
        payload as UpdateRouteRequest
      )
    } else {
      await publisherApi.routes.create(payload as CreateRouteRequest)
    }
    closeModal()
    await loadRoutes()
  } catch (err) {
    const axiosError = err as { response?: { data?: { error?: string } } }
    modalError.value = axiosError.response?.data?.error || 'Failed to save route'
  } finally {
    saving.value = false
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

onMounted(() => {
  loadRoutes()
  loadSources()
  loadChannels()
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-3xl font-bold tracking-tight">
          Routes
        </h1>
        <p class="text-muted-foreground">
          Configure how content flows to channels
        </p>
      </div>
      <Button @click="openCreateModal">
        <Plus class="mr-2 h-4 w-4" />
        New Route
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

    <Card v-else-if="routes.length === 0">
      <CardContent class="flex flex-col items-center justify-center py-12">
        <GitBranch class="h-12 w-12 text-muted-foreground mb-4" />
        <h3 class="text-lg font-medium mb-2">
          No routes configured
        </h3>
        <p class="text-muted-foreground mb-4">
          Create routes to publish content to channels.
        </p>
        <Button @click="openCreateModal">
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
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Source
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Channel
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Min Quality
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Topics
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Status
              </th>
              <th class="px-6 py-3 text-right text-xs font-medium text-muted-foreground uppercase">
                Actions
              </th>
            </tr>
          </thead>
          <tbody class="divide-y">
            <tr
              v-for="route in routes"
              :key="route.id"
              class="hover:bg-muted/50"
            >
              <td class="px-6 py-4 text-sm font-medium">
                {{ route.source_name }}
              </td>
              <td class="px-6 py-4 text-sm text-primary">
                {{ route.channel_name }}
              </td>
              <td class="px-6 py-4 text-sm text-muted-foreground">
                {{ route.min_quality_score }}/100
              </td>
              <td class="px-6 py-4">
                <div class="flex gap-1 flex-wrap">
                  <Badge
                    v-for="topic in route.topics?.slice(0, 3)"
                    :key="topic"
                    variant="outline"
                    class="text-xs"
                  >
                    {{ topic }}
                  </Badge>
                  <Badge
                    v-if="(route.topics?.length || 0) > 3"
                    variant="outline"
                    class="text-xs"
                  >
                    +{{ route.topics!.length - 3 }}
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
                  <Button
                    variant="ghost"
                    size="icon"
                    @click="openEditModal(route)"
                  >
                    <Pencil class="h-4 w-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    @click="deleteRoute(route.id)"
                  >
                    <Trash2 class="h-4 w-4 text-destructive" />
                  </Button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </CardContent>
    </Card>

    <!-- Create/Edit Modal -->
    <div
      v-if="showModal"
      class="fixed inset-0 z-50 flex items-center justify-center"
    >
      <!-- Backdrop -->
      <div
        class="fixed inset-0 bg-black/50"
        @click="closeModal"
      />

      <!-- Modal Content -->
      <Card class="relative z-10 w-full max-w-md mx-4">
        <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-4">
          <CardTitle>
            {{ isEditing ? 'Edit Route' : 'Create Route' }}
          </CardTitle>
          <Button
            variant="ghost"
            size="icon"
            @click="closeModal"
          >
            <X class="h-4 w-4" />
          </Button>
        </CardHeader>
        <CardContent>
          <!-- Error Alert -->
          <div
            v-if="modalError"
            class="mb-4 p-3 rounded-md bg-destructive/10 text-destructive text-sm"
          >
            {{ modalError }}
          </div>

          <form
            class="space-y-4"
            @submit.prevent="saveRoute"
          >
            <!-- Source Select -->
            <div class="space-y-2">
              <label class="text-sm font-medium">Source *</label>
              <select
                v-model="formData.source_id"
                required
                class="w-full h-10 rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2"
              >
                <option :value="0">
                  Select a source...
                </option>
                <option
                  v-for="source in sources"
                  :key="source.id"
                  :value="source.id"
                >
                  {{ source.name }} ({{ source.index_pattern }})
                </option>
              </select>
            </div>

            <!-- Channel Select -->
            <div class="space-y-2">
              <label class="text-sm font-medium">Channel *</label>
              <select
                v-model="formData.channel_id"
                required
                class="w-full h-10 rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2"
              >
                <option :value="0">
                  Select a channel...
                </option>
                <option
                  v-for="channel in channels"
                  :key="channel.id"
                  :value="channel.id"
                >
                  {{ channel.name }}{{ !channel.enabled ? ' (disabled)' : '' }}
                </option>
              </select>
              <div
                v-if="channels.length === 0"
                class="p-3 rounded-md bg-yellow-500/10 text-yellow-700 dark:text-yellow-400 text-sm"
              >
                <p class="mb-2">
                  No channels available. Create one in the Channels page.
                </p>
                <router-link
                  to="/distribution/channels"
                  class="text-primary hover:underline font-medium"
                  @click="closeModal"
                >
                  Go to Channels →
                </router-link>
              </div>
            </div>

            <!-- Min Quality Score -->
            <div class="space-y-2">
              <label class="text-sm font-medium">Minimum Quality Score</label>
              <Input
                v-model.number="formData.min_quality_score"
                type="number"
                min="0"
                max="100"
              />
              <p class="text-xs text-muted-foreground">
                Only publish articles with quality score >= this value (0-100)
              </p>
            </div>

            <!-- Topics -->
            <div class="space-y-2">
              <label class="text-sm font-medium">Topics</label>
              <Input
                v-model="topicsInput"
                type="text"
                placeholder="e.g., crime, news, local"
              />
              <p class="text-xs text-muted-foreground">
                Comma-separated list of topics to filter (leave empty for all topics)
              </p>
            </div>

            <!-- Enabled Checkbox -->
            <div class="flex items-center space-x-2">
              <input
                id="enabled"
                v-model="formData.enabled"
                type="checkbox"
                class="h-4 w-4 rounded border-input"
              >
              <label
                for="enabled"
                class="text-sm font-medium"
              >Enabled</label>
            </div>

            <!-- Actions -->
            <div class="flex justify-end gap-3 pt-4">
              <Button
                type="button"
                variant="outline"
                @click="closeModal"
              >
                Cancel
              </Button>
              <Button
                type="submit"
                :disabled="saving || formData.source_id === 0 || formData.channel_id === 0"
              >
                <Loader2
                  v-if="saving"
                  class="mr-2 h-4 w-4 animate-spin"
                />
                {{ saving ? 'Saving...' : 'Save' }}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
