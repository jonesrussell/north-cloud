import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { sourcesApi } from '@/api/client'

export interface Source {
  id: string
  name: string
  url: string
  is_enabled: boolean
  created_at: string
  updated_at?: string
  crawl_config?: {
    selectors?: Record<string, unknown>
    interval_minutes?: number
  }
}

export const useSourcesStore = defineStore('sources', () => {
  // State
  const items = ref<Source[]>([])
  const selectedSource = ref<Source | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)

  // Getters
  const enabledSources = computed(() => items.value.filter((s) => s.is_enabled))

  const disabledSources = computed(() => items.value.filter((s) => !s.is_enabled))

  const sourceCount = computed(() => items.value.length)

  const enabledCount = computed(() => enabledSources.value.length)

  // For dropdowns and filters
  const sourceOptions = computed(() =>
    items.value.map((s) => ({ id: s.id, name: s.name, url: s.url }))
  )

  // Actions
  async function fetchSources() {
    loading.value = true
    error.value = null

    try {
      const response = await sourcesApi.list()
      const data = response.data?.sources || response.data || []
      items.value = data
    } catch (err) {
      error.value = 'Failed to load sources. Please check if the source-manager service is running.'
      console.error('Failed to fetch sources:', err)
    } finally {
      loading.value = false
    }
  }

  async function fetchSource(id: string) {
    loading.value = true
    error.value = null

    try {
      const response = await sourcesApi.get(id)
      selectedSource.value = response.data
      return response.data
    } catch (err) {
      error.value = 'Failed to load source details.'
      console.error('Failed to fetch source:', err)
      return null
    } finally {
      loading.value = false
    }
  }

  async function createSource(data: Partial<Source>) {
    try {
      const response = await sourcesApi.create(data)
      await fetchSources() // Refresh list
      return response.data
    } catch (err) {
      error.value = 'Failed to create source.'
      console.error('Failed to create source:', err)
      throw err
    }
  }

  async function updateSource(id: string, data: Partial<Source>) {
    try {
      const response = await sourcesApi.update(id, data)
      await fetchSources() // Refresh list
      if (selectedSource.value?.id === id) {
        await fetchSource(id)
      }
      return response.data
    } catch (err) {
      error.value = 'Failed to update source.'
      console.error('Failed to update source:', err)
      throw err
    }
  }

  async function deleteSource(id: string) {
    try {
      await sourcesApi.delete(id)
      await fetchSources() // Refresh list
      if (selectedSource.value?.id === id) {
        selectedSource.value = null
      }
    } catch (err) {
      error.value = 'Failed to delete source.'
      console.error('Failed to delete source:', err)
      throw err
    }
  }

  async function toggleSourceEnabled(id: string) {
    const source = items.value.find((s) => s.id === id)
    if (!source) return

    await updateSource(id, { is_enabled: !source.is_enabled })
  }

  function getSourceById(id: string): Source | undefined {
    return items.value.find((s) => s.id === id)
  }

  function getSourceByName(name: string): Source | undefined {
    return items.value.find((s) => s.name.toLowerCase() === name.toLowerCase())
  }

  function $reset() {
    items.value = []
    selectedSource.value = null
    loading.value = false
    error.value = null
  }

  return {
    // State
    items,
    selectedSource,
    loading,
    error,

    // Getters
    enabledSources,
    disabledSources,
    sourceCount,
    enabledCount,
    sourceOptions,

    // Actions
    fetchSources,
    fetchSource,
    createSource,
    updateSource,
    deleteSource,
    toggleSourceEnabled,
    getSourceById,
    getSourceByName,

    $reset,
  }
})
