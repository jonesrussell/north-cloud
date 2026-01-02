import { ref, computed } from 'vue'

export interface BulkOperationsOptions {
  onSuccess?: (action: string, count: number) => void
  onError?: (action: string, error: Error) => void
}

export function useBulkOperations(options: BulkOperationsOptions = {}) {
  const selectedIds = ref<Set<string>>(new Set())
  const selectAll = ref(false)

  const selectedCount = computed(() => selectedIds.value.size)
  const selectedArray = computed(() => Array.from(selectedIds.value))
  const hasSelection = computed(() => selectedIds.value.size > 0)

  function toggleSelectAll(items: Array<{ id: string }>) {
    if (selectAll.value) {
      selectedIds.value.clear()
      selectAll.value = false
    } else {
      items.forEach(item => selectedIds.value.add(item.id))
      selectAll.value = true
    }
  }

  function toggleItem(id: string) {
    if (selectedIds.value.has(id)) {
      selectedIds.value.delete(id)
      selectAll.value = false
    } else {
      selectedIds.value.add(id)
    }
  }

  function isSelected(id: string): boolean {
    return selectedIds.value.has(id)
  }

  function clearSelection() {
    selectedIds.value.clear()
    selectAll.value = false
  }

  async function performBulkAction<T>(
    action: string,
    apiCall: (ids: string[]) => Promise<T>
  ): Promise<T | null> {
    const ids = selectedArray.value

    if (ids.length === 0) {
      throw new Error('No items selected')
    }

    try {
      const result = await apiCall(ids)
      options.onSuccess?.(action, ids.length)
      clearSelection()
      return result
    } catch (error) {
      options.onError?.(action, error as Error)
      throw error
    }
  }

  return {
    selectedIds: selectedArray,
    selectedCount,
    hasSelection,
    selectAll,
    toggleSelectAll,
    toggleItem,
    isSelected,
    clearSelection,
    performBulkAction,
  }
}
