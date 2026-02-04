<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  Database,
  RefreshCw,
  AlertTriangle,
  Loader2,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import {
  useIndexes,
  IndexStatsCards,
  IndexesFilterBar,
  IndexesTable,
} from '@/features/intelligence'
import type { Index } from '@/types/indexManager'

const router = useRouter()
const indexes = useIndexes()

// Delete confirmation state
const deleteModalOpen = ref(false)
const indexToDelete = ref<Index | null>(null)
const deleteError = ref<string | null>(null)

function handleView(index: Index) {
  router.push({ name: 'intelligence-index-detail', params: { index_name: index.name } })
}

function handleDeleteClick(index: Index) {
  indexToDelete.value = index
  deleteError.value = null
  deleteModalOpen.value = true
}

function cancelDelete() {
  deleteModalOpen.value = false
  indexToDelete.value = null
  deleteError.value = null
}

async function confirmDelete() {
  if (!indexToDelete.value) return

  try {
    deleteError.value = null
    await indexes.deleteIndex(indexToDelete.value.name)
    deleteModalOpen.value = false
    indexToDelete.value = null
  } catch {
    deleteError.value = 'Failed to delete index. Please try again.'
  }
}
</script>

<template>
  <div class="min-h-screen">
    <!-- Header Section -->
    <header class="mb-6">
      <div class="flex items-start justify-between">
        <div>
          <div class="mb-2 flex items-center gap-3">
            <div class="rounded-lg border bg-blue-500/10 p-2">
              <Database class="h-6 w-6 text-blue-500" />
            </div>
            <h1 class="text-2xl font-semibold tracking-tight">
              Index Explorer
            </h1>
          </div>
          <p class="ml-[52px] text-sm text-muted-foreground">
            Browse and manage Elasticsearch indexes with document search and filtering
          </p>
        </div>
        <Button
          variant="outline"
          size="sm"
          :disabled="indexes.isLoading.value"
          @click="indexes.refetch()"
        >
          <RefreshCw
            class="mr-2 h-4 w-4 transition-transform"
            :class="{ 'animate-spin': indexes.isFetching.value }"
          />
          Refresh
        </Button>
      </div>
    </header>

    <!-- Stats Cards -->
    <div class="mb-6">
      <IndexStatsCards
        :stats="indexes.stats.value"
        :loading="indexes.statsLoading.value"
      />
    </div>

    <!-- Filter Bar -->
    <div class="mb-6">
      <IndexesFilterBar />
    </div>

    <!-- Error State -->
    <div
      v-if="indexes.hasError.value"
      class="rounded-xl border border-rose-500/30 bg-rose-500/10 p-8 text-center"
    >
      <AlertTriangle class="mx-auto mb-4 h-12 w-12 text-rose-400" />
      <p class="font-medium text-rose-400">
        Unable to load Elasticsearch indexes.
      </p>
      <Button
        variant="outline"
        size="sm"
        class="mt-4 border-rose-500/30 hover:bg-rose-500/10"
        @click="indexes.refetch()"
      >
        Try Again
      </Button>
    </div>

    <!-- Table -->
    <div v-else>
      <IndexesTable
        @view="handleView"
        @delete="handleDeleteClick"
      />
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
          <div class="relative z-10 w-full max-w-md rounded-2xl border bg-card shadow-xl">
            <div class="p-6">
              <div class="flex items-start gap-4">
                <div class="flex h-12 w-12 shrink-0 items-center justify-center rounded-full bg-rose-500/10">
                  <AlertTriangle class="h-6 w-6 text-rose-400" />
                </div>
                <div class="min-w-0 flex-1">
                  <h3 class="mb-2 text-lg font-semibold">
                    Delete Index
                  </h3>
                  <p class="mb-2 text-sm text-muted-foreground">
                    Are you sure you want to delete:
                  </p>
                  <code class="mb-4 block break-all rounded-lg bg-rose-500/10 px-3 py-2 font-mono text-sm font-medium text-rose-400">
                    {{ indexToDelete?.name }}
                  </code>
                  <p class="text-sm text-muted-foreground">
                    This action cannot be undone. All documents will be permanently deleted.
                  </p>

                  <div
                    v-if="deleteError"
                    class="mt-4 rounded-lg bg-rose-500/10 px-3 py-2 text-sm text-rose-400"
                  >
                    {{ deleteError }}
                  </div>

                  <div class="mt-6 flex justify-end gap-3">
                    <Button
                      variant="outline"
                      :disabled="indexes.isDeleting.value"
                      @click="cancelDelete"
                    >
                      Cancel
                    </Button>
                    <Button
                      variant="destructive"
                      :disabled="indexes.isDeleting.value"
                      @click="confirmDelete"
                    >
                      <Loader2
                        v-if="indexes.isDeleting.value"
                        class="mr-2 h-4 w-4 animate-spin"
                      />
                      {{ indexes.isDeleting.value ? 'Deleting...' : 'Delete Index' }}
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
/* Modal transitions */
.modal-enter-active,
.modal-leave-active {
  transition: all 0.2s ease;
}

.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}
</style>
