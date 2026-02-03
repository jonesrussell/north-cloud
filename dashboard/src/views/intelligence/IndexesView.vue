<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import {
  Loader2,
  Database,
  RefreshCw,
  Trash2,
  AlertTriangle,
  HardDrive,
  FileText,
  Activity,
  Layers,
  Search,
  Filter
} from 'lucide-vue-next'
import { indexManagerApi } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import type { Index } from '@/types/indexManager'

interface DisplayIndex {
  name: string
  document_count: number
  size: string
  sizeBytes: number
  health: string
  type: string
}

const router = useRouter()
const loading = ref(true)
const error = ref<string | null>(null)
const indexes = ref<DisplayIndex[]>([])
const searchQuery = ref('')
const filterType = ref<string | null>(null)

// Delete confirmation state
const deleteModalOpen = ref(false)
const indexToDelete = ref<string | null>(null)
const deleting = ref(false)
const deleteError = ref<string | null>(null)

// Parse size string to bytes for sorting
const parseSizeToBytes = (size: string): number => {
  const match = size.match(/^([\d.]+)\s*([KMGT]?B)$/i)
  if (!match) return 0
  const value = parseFloat(match[1])
  const unit = match[2].toUpperCase()
  const multipliers: Record<string, number> = {
    'B': 1,
    'KB': 1024,
    'MB': 1024 * 1024,
    'GB': 1024 * 1024 * 1024,
    'TB': 1024 * 1024 * 1024 * 1024
  }
  return value * (multipliers[unit] || 1)
}

const loadIndexes = async () => {
  try {
    loading.value = true
    error.value = null
    const response = await indexManagerApi.indexes.list()
    const rawIndices: Index[] = response.data?.indices || []
    indexes.value = rawIndices.map((idx) => ({
      name: idx.name,
      document_count: idx.document_count || 0,
      size: idx.size || '0 B',
      sizeBytes: parseSizeToBytes(idx.size || '0 B'),
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

// Filtered and sorted indexes
const filteredIndexes = computed(() => {
  let result = [...indexes.value]

  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(idx => idx.name.toLowerCase().includes(query))
  }

  if (filterType.value) {
    result = result.filter(idx => idx.type === filterType.value)
  }

  return result
})

// Statistics
const stats = computed(() => {
  const total = indexes.value.length
  const totalDocs = indexes.value.reduce((sum, idx) => sum + idx.document_count, 0)
  const totalSize = indexes.value.reduce((sum, idx) => sum + idx.sizeBytes, 0)
  const healthCounts = {
    green: indexes.value.filter(i => i.health === 'green').length,
    yellow: indexes.value.filter(i => i.health === 'yellow').length,
    red: indexes.value.filter(i => i.health === 'red').length,
  }
  return { total, totalDocs, totalSize, healthCounts }
})

const formatSize = (bytes: number): string => {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

const formatNumber = (num: number): string => {
  if (num >= 1000000) return (num / 1000000).toFixed(1) + 'M'
  if (num >= 1000) return (num / 1000).toFixed(1) + 'K'
  return num.toString()
}

const uniqueTypes = computed(() => {
  const types = new Set(indexes.value.map(i => i.type))
  return Array.from(types).filter(t => t !== 'unknown')
})

const getHealthColor = (health: string) => {
  switch (health) {
    case 'green': return 'text-emerald-400'
    case 'yellow': return 'text-amber-400'
    case 'red': return 'text-rose-400'
    default: return 'text-slate-500'
  }
}

const getHealthBg = (health: string) => {
  switch (health) {
    case 'green': return 'bg-emerald-500/20'
    case 'yellow': return 'bg-amber-500/20'
    case 'red': return 'bg-rose-500/20'
    default: return 'bg-slate-500/20'
  }
}

const getHealthGlow = (health: string) => {
  switch (health) {
    case 'green': return 'shadow-emerald-500/30'
    case 'yellow': return 'shadow-amber-500/30'
    case 'red': return 'shadow-rose-500/30'
    default: return 'shadow-slate-500/30'
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
  <div class="min-h-screen indexes-view">
    <!-- Header Section -->
    <header class="mb-8">
      <div class="flex items-start justify-between">
        <div>
          <div class="flex items-center gap-3 mb-2">
            <div class="p-2 rounded-lg bg-gradient-to-br from-blue-500/20 to-cyan-500/20 border border-blue-500/30">
              <Database class="h-6 w-6 text-blue-400" />
            </div>
            <h1 class="text-2xl font-semibold tracking-tight text-foreground">
              Index Observatory
            </h1>
          </div>
          <p class="text-muted-foreground text-sm ml-[52px]">
            Monitor and manage Elasticsearch indexes across your data pipeline
          </p>
        </div>
        <Button
          variant="outline"
          size="sm"
          class="border-border/50 hover:border-blue-500/50 hover:bg-blue-500/10 transition-all duration-300"
          :disabled="loading"
          @click="loadIndexes"
        >
          <RefreshCw
            class="mr-2 h-4 w-4 transition-transform"
            :class="{ 'animate-spin': loading }"
          />
          Refresh
        </Button>
      </div>
    </header>

    <!-- Stats Cards -->
    <div
      v-if="!loading && !error && indexes.length > 0"
      class="grid grid-cols-2 md:grid-cols-4 gap-4 mb-8"
    >
      <div class="stat-card group">
        <div class="flex items-center gap-3">
          <div class="p-2 rounded-lg bg-blue-500/10 group-hover:bg-blue-500/20 transition-colors">
            <Layers class="h-5 w-5 text-blue-400" />
          </div>
          <div>
            <div class="text-2xl font-bold tabular-nums text-foreground">
              {{ stats.total }}
            </div>
            <div class="text-xs text-muted-foreground uppercase tracking-wider">
              Indexes
            </div>
          </div>
        </div>
      </div>

      <div class="stat-card group">
        <div class="flex items-center gap-3">
          <div class="p-2 rounded-lg bg-violet-500/10 group-hover:bg-violet-500/20 transition-colors">
            <FileText class="h-5 w-5 text-violet-400" />
          </div>
          <div>
            <div class="text-2xl font-bold tabular-nums text-foreground">
              {{ formatNumber(stats.totalDocs) }}
            </div>
            <div class="text-xs text-muted-foreground uppercase tracking-wider">
              Documents
            </div>
          </div>
        </div>
      </div>

      <div class="stat-card group">
        <div class="flex items-center gap-3">
          <div class="p-2 rounded-lg bg-cyan-500/10 group-hover:bg-cyan-500/20 transition-colors">
            <HardDrive class="h-5 w-5 text-cyan-400" />
          </div>
          <div>
            <div class="text-2xl font-bold tabular-nums text-foreground">
              {{ formatSize(stats.totalSize) }}
            </div>
            <div class="text-xs text-muted-foreground uppercase tracking-wider">
              Total Size
            </div>
          </div>
        </div>
      </div>

      <div class="stat-card group">
        <div class="flex items-center gap-3">
          <div class="p-2 rounded-lg bg-emerald-500/10 group-hover:bg-emerald-500/20 transition-colors">
            <Activity class="h-5 w-5 text-emerald-400" />
          </div>
          <div class="flex items-center gap-2">
            <div class="flex items-center gap-1">
              <span class="inline-block w-2 h-2 rounded-full bg-emerald-400 animate-pulse" />
              <span class="text-sm font-semibold text-emerald-400">{{ stats.healthCounts.green }}</span>
            </div>
            <div class="flex items-center gap-1">
              <span class="inline-block w-2 h-2 rounded-full bg-amber-400" />
              <span class="text-sm font-semibold text-amber-400">{{ stats.healthCounts.yellow }}</span>
            </div>
            <div class="flex items-center gap-1">
              <span class="inline-block w-2 h-2 rounded-full bg-rose-400" />
              <span class="text-sm font-semibold text-rose-400">{{ stats.healthCounts.red }}</span>
            </div>
          </div>
        </div>
        <div class="text-xs text-muted-foreground uppercase tracking-wider mt-1 ml-[44px]">
          Health Status
        </div>
      </div>
    </div>

    <!-- Search and Filter Bar -->
    <div
      v-if="!loading && !error && indexes.length > 0"
      class="flex flex-col sm:flex-row gap-3 mb-6"
    >
      <div class="relative flex-1">
        <Search class="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
        <Input
          v-model="searchQuery"
          type="text"
          placeholder="Search indexes..."
          class="pl-10 bg-card/50 border-border/50 focus:border-blue-500/50 focus:ring-blue-500/20"
        />
      </div>
      <div class="flex gap-2">
        <Button
          v-for="type in ['all', ...uniqueTypes]"
          :key="type"
          size="sm"
          :variant="(type === 'all' && !filterType) || filterType === type ? 'default' : 'outline'"
          class="capitalize transition-all"
          :class="{
            'bg-blue-600 hover:bg-blue-700': (type === 'all' && !filterType) || filterType === type,
            'border-border/50 hover:border-blue-500/30': !((type === 'all' && !filterType) || filterType === type)
          }"
          @click="filterType = type === 'all' ? null : type"
        >
          <Filter v-if="type !== 'all'" class="h-3 w-3 mr-1" />
          {{ type === 'all' ? 'All Types' : type.replace(/_/g, ' ') }}
        </Button>
      </div>
    </div>

    <!-- Loading State -->
    <div
      v-if="loading"
      class="flex flex-col items-center justify-center py-24"
    >
      <div class="relative">
        <div class="absolute inset-0 rounded-full bg-blue-500/20 animate-ping" />
        <Loader2 class="h-12 w-12 animate-spin text-blue-400 relative" />
      </div>
      <p class="mt-4 text-muted-foreground text-sm">Loading indexes...</p>
    </div>

    <!-- Error State -->
    <div
      v-else-if="error"
      class="rounded-xl border border-rose-500/30 bg-rose-500/10 p-8 text-center"
    >
      <AlertTriangle class="h-12 w-12 text-rose-400 mx-auto mb-4" />
      <p class="text-rose-400 font-medium">{{ error }}</p>
      <Button
        variant="outline"
        size="sm"
        class="mt-4 border-rose-500/30 hover:bg-rose-500/10"
        @click="loadIndexes"
      >
        Try Again
      </Button>
    </div>

    <!-- Empty State -->
    <div
      v-else-if="indexes.length === 0"
      class="rounded-xl border border-border/50 bg-card/30 p-12 text-center"
    >
      <div class="inline-flex p-4 rounded-full bg-muted/50 mb-4">
        <Database class="h-12 w-12 text-muted-foreground" />
      </div>
      <h3 class="text-lg font-medium text-foreground mb-2">No indexes found</h3>
      <p class="text-muted-foreground text-sm max-w-md mx-auto">
        Indexes will be created automatically when content is crawled and classified.
      </p>
    </div>

    <!-- Index Grid -->
    <div
      v-else-if="filteredIndexes.length > 0"
      class="grid gap-3"
    >
      <TransitionGroup name="list">
        <div
          v-for="(index, i) in filteredIndexes"
          :key="index.name"
          class="index-card group"
          :style="{ animationDelay: `${i * 30}ms` }"
          @click="viewIndex(index.name)"
        >
          <!-- Health Indicator -->
          <div class="flex items-center gap-4 min-w-0">
            <div
              class="relative flex-shrink-0 w-3 h-3 rounded-full transition-all duration-300"
              :class="[getHealthBg(index.health), getHealthColor(index.health)]"
            >
              <span
                v-if="index.health === 'green'"
                class="absolute inset-0 rounded-full animate-ping opacity-75"
                :class="getHealthBg(index.health)"
              />
            </div>

            <!-- Index Name -->
            <div class="min-w-0 flex-1">
              <h3 class="font-mono text-sm font-medium text-foreground truncate group-hover:text-blue-400 transition-colors">
                {{ index.name }}
              </h3>
              <span class="text-xs text-muted-foreground capitalize">
                {{ index.type.replace(/_/g, ' ') }}
              </span>
            </div>
          </div>

          <!-- Metrics -->
          <div class="flex items-center gap-6">
            <div class="text-right">
              <div class="text-sm font-semibold tabular-nums text-foreground">
                {{ index.document_count.toLocaleString() }}
              </div>
              <div class="text-xs text-muted-foreground">docs</div>
            </div>
            <div class="text-right">
              <div class="text-sm font-semibold tabular-nums text-foreground">
                {{ index.size }}
              </div>
              <div class="text-xs text-muted-foreground">size</div>
            </div>
            <div
              class="px-2 py-1 rounded text-xs font-medium capitalize"
              :class="[getHealthBg(index.health), getHealthColor(index.health)]"
            >
              {{ index.health }}
            </div>
            <Button
              variant="ghost"
              size="icon"
              class="h-8 w-8 opacity-0 group-hover:opacity-100 transition-opacity hover:bg-rose-500/10 hover:text-rose-400"
              title="Delete index"
              @click.stop="confirmDelete(index.name)"
            >
              <Trash2 class="h-4 w-4" />
            </Button>
          </div>
        </div>
      </TransitionGroup>
    </div>

    <!-- No Results -->
    <div
      v-else
      class="rounded-xl border border-border/50 bg-card/30 p-8 text-center"
    >
      <Search class="h-8 w-8 text-muted-foreground mx-auto mb-3" />
      <p class="text-muted-foreground text-sm">
        No indexes match your search criteria
      </p>
    </div>

    <!-- Delete Confirmation Modal -->
    <Teleport to="body">
      <Transition name="modal">
        <div
          v-if="deleteModalOpen"
          class="fixed inset-0 z-50 flex items-center justify-center p-4"
        >
          <!-- Backdrop -->
          <div
            class="fixed inset-0 bg-black/60 backdrop-blur-sm"
            @click="cancelDelete"
          />

          <!-- Modal -->
          <div class="modal-content relative z-10 w-full max-w-md">
            <div class="p-6">
              <div class="flex items-start gap-4">
                <div class="flex-shrink-0 w-12 h-12 rounded-full bg-rose-500/10 flex items-center justify-center">
                  <AlertTriangle class="h-6 w-6 text-rose-400" />
                </div>
                <div class="flex-1 min-w-0">
                  <h3 class="text-lg font-semibold text-foreground mb-2">
                    Delete Index
                  </h3>
                  <p class="text-sm text-muted-foreground mb-2">
                    Are you sure you want to delete:
                  </p>
                  <code class="block text-sm font-mono font-medium text-rose-400 bg-rose-500/10 px-3 py-2 rounded-lg mb-4 break-all">
                    {{ indexToDelete }}
                  </code>
                  <p class="text-sm text-muted-foreground">
                    This action cannot be undone. All documents will be permanently deleted.
                  </p>

                  <div
                    v-if="deleteError"
                    class="mt-4 text-sm text-rose-400 bg-rose-500/10 px-3 py-2 rounded-lg"
                  >
                    {{ deleteError }}
                  </div>

                  <div class="flex justify-end gap-3 mt-6">
                    <Button
                      variant="outline"
                      :disabled="deleting"
                      class="border-border/50"
                      @click="cancelDelete"
                    >
                      Cancel
                    </Button>
                    <Button
                      variant="destructive"
                      :disabled="deleting"
                      class="bg-rose-600 hover:bg-rose-700"
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
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<style scoped>
.indexes-view {
  --card-glow: 0 0 0 1px rgba(59, 130, 246, 0);
}

.stat-card {
  @apply relative p-4 rounded-xl border border-border/50 bg-card/50 backdrop-blur-sm;
  @apply transition-all duration-300 ease-out;
  @apply hover:border-blue-500/30 hover:bg-card/80;
  box-shadow: var(--card-glow);
}

.stat-card:hover {
  --card-glow: 0 0 20px -5px rgba(59, 130, 246, 0.15);
}

.index-card {
  @apply relative flex items-center justify-between gap-4 p-4 rounded-xl;
  @apply border border-border/50 bg-card/30 backdrop-blur-sm;
  @apply cursor-pointer transition-all duration-200 ease-out;
  @apply hover:border-blue-500/40 hover:bg-card/60;
  animation: slideIn 0.3s ease-out backwards;
}

.index-card:hover {
  box-shadow: 0 0 30px -10px rgba(59, 130, 246, 0.2);
  transform: translateX(2px);
}

@keyframes slideIn {
  from {
    opacity: 0;
    transform: translateX(-10px);
  }
  to {
    opacity: 1;
    transform: translateX(0);
  }
}

.modal-content {
  @apply rounded-2xl border border-border/50 bg-card backdrop-blur-xl;
  box-shadow:
    0 0 0 1px rgba(255, 255, 255, 0.05),
    0 25px 50px -12px rgba(0, 0, 0, 0.5),
    0 0 100px -20px rgba(59, 130, 246, 0.2);
  animation: modalIn 0.2s ease-out;
}

@keyframes modalIn {
  from {
    opacity: 0;
    transform: scale(0.95) translateY(10px);
  }
  to {
    opacity: 1;
    transform: scale(1) translateY(0);
  }
}

/* List transitions */
.list-enter-active,
.list-leave-active {
  transition: all 0.3s ease;
}

.list-enter-from {
  opacity: 0;
  transform: translateX(-20px);
}

.list-leave-to {
  opacity: 0;
  transform: translateX(20px);
}

/* Modal transitions */
.modal-enter-active,
.modal-leave-active {
  transition: all 0.2s ease;
}

.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}

.modal-enter-from .modal-content,
.modal-leave-to .modal-content {
  transform: scale(0.95) translateY(10px);
}
</style>
