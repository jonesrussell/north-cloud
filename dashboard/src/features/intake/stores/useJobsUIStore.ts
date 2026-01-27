/**
 * Jobs UI Store
 *
 * Manages UI state for jobs: modals, selections, bulk actions, and preferences.
 * This is pure CLIENT-SIDE state with no server interaction.
 */

import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { Job } from '@/types/crawler'

// ============================================================================
// Types
// ============================================================================

export type JobViewMode = 'table' | 'grid' | 'compact'

export interface BulkActionState {
  isActive: boolean
  selectedIds: Set<string>
  action: 'pause' | 'resume' | 'cancel' | 'delete' | null
}

export interface JobModalState {
  create: boolean
  edit: boolean
  delete: boolean
  details: boolean
  logs: boolean
}

export type JobAction = 'pause' | 'resume' | 'cancel' | 'delete' | 'retry'

// ============================================================================
// Store
// ============================================================================

export const useJobsUIStore = defineStore('jobs-ui', () => {
  // ---------------------------------------------------------------------------
  // State: View Preferences
  // ---------------------------------------------------------------------------

  const viewMode = ref<JobViewMode>('table')
  const compactMode = ref(false)
  const showFilters = ref(true)

  // ---------------------------------------------------------------------------
  // State: Selection
  // ---------------------------------------------------------------------------

  const selectedJobId = ref<string | null>(null)
  const selectedJob = ref<Job | null>(null)

  // ---------------------------------------------------------------------------
  // State: Bulk Selection
  // ---------------------------------------------------------------------------

  const bulkAction = ref<BulkActionState>({
    isActive: false,
    selectedIds: new Set<string>(),
    action: null,
  })

  // ---------------------------------------------------------------------------
  // State: Modal Visibility
  // ---------------------------------------------------------------------------

  const modals = ref<JobModalState>({
    create: false,
    edit: false,
    delete: false,
    details: false,
    logs: false,
  })

  // ---------------------------------------------------------------------------
  // State: Job Action Confirmation
  // ---------------------------------------------------------------------------

  const actionConfirmation = ref<{
    jobId: string | null
    action: JobAction | null
  }>({
    jobId: null,
    action: null,
  })

  // ---------------------------------------------------------------------------
  // State: Action In Progress
  // ---------------------------------------------------------------------------

  const actionInProgressJobId = ref<string | null>(null)

  // ---------------------------------------------------------------------------
  // Computed
  // ---------------------------------------------------------------------------

  const hasSelection = computed(() => selectedJobId.value !== null)

  const bulkSelectionCount = computed(() => bulkAction.value.selectedIds.size)

  const hasBulkSelection = computed(() => bulkSelectionCount.value > 0)

  const isBulkActionActive = computed(() => bulkAction.value.isActive)

  const hasOpenModal = computed(() => Object.values(modals.value).some(Boolean))

  const isActionInProgress = computed(() => actionInProgressJobId.value !== null)

  const isPendingConfirmation = computed(() => actionConfirmation.value.action !== null)

  // ---------------------------------------------------------------------------
  // Actions: View Preferences
  // ---------------------------------------------------------------------------

  function setViewMode(mode: JobViewMode) {
    viewMode.value = mode
  }

  function toggleCompactMode() {
    compactMode.value = !compactMode.value
  }

  function toggleFilters() {
    showFilters.value = !showFilters.value
  }

  // ---------------------------------------------------------------------------
  // Actions: Selection
  // ---------------------------------------------------------------------------

  function selectJob(jobId: string, job?: Job) {
    selectedJobId.value = jobId
    selectedJob.value = job || null
  }

  function clearSelection() {
    selectedJobId.value = null
    selectedJob.value = null
  }

  function isJobSelected(jobId: string): boolean {
    return selectedJobId.value === jobId
  }

  // ---------------------------------------------------------------------------
  // Actions: Bulk Selection
  // ---------------------------------------------------------------------------

  function startBulkAction(action: BulkActionState['action']) {
    bulkAction.value.isActive = true
    bulkAction.value.action = action
    bulkAction.value.selectedIds.clear()
  }

  function toggleBulkSelect(jobId: string) {
    const selectedIds = bulkAction.value.selectedIds
    if (selectedIds.has(jobId)) {
      selectedIds.delete(jobId)
    } else {
      selectedIds.add(jobId)
    }
  }

  function selectAllJobs(jobIds: string[]) {
    bulkAction.value.selectedIds = new Set(jobIds)
  }

  function clearBulkSelection() {
    bulkAction.value.selectedIds.clear()
  }

  function cancelBulkAction() {
    bulkAction.value.isActive = false
    bulkAction.value.action = null
    bulkAction.value.selectedIds.clear()
  }

  function isBulkSelected(jobId: string): boolean {
    return bulkAction.value.selectedIds.has(jobId)
  }

  function getBulkSelectedIds(): string[] {
    return Array.from(bulkAction.value.selectedIds)
  }

  // ---------------------------------------------------------------------------
  // Actions: Modals
  // ---------------------------------------------------------------------------

  function openModal(modalName: keyof JobModalState) {
    modals.value[modalName] = true
  }

  function closeModal(modalName: keyof JobModalState) {
    modals.value[modalName] = false
  }

  function closeAllModals() {
    const keys = Object.keys(modals.value) as Array<keyof JobModalState>
    keys.forEach((key) => {
      modals.value[key] = false
    })
  }

  // ---------------------------------------------------------------------------
  // Actions: Job Action Confirmation
  // ---------------------------------------------------------------------------

  function requestActionConfirmation(jobId: string, action: JobAction) {
    actionConfirmation.value = { jobId, action }
  }

  function confirmAction(): { jobId: string; action: JobAction } | null {
    if (!actionConfirmation.value.jobId || !actionConfirmation.value.action) {
      return null
    }

    const result = {
      jobId: actionConfirmation.value.jobId,
      action: actionConfirmation.value.action,
    }

    actionConfirmation.value = { jobId: null, action: null }
    return result
  }

  function cancelActionConfirmation() {
    actionConfirmation.value = { jobId: null, action: null }
  }

  // ---------------------------------------------------------------------------
  // Actions: Action In Progress
  // ---------------------------------------------------------------------------

  function setActionInProgress(jobId: string | null) {
    actionInProgressJobId.value = jobId
  }

  function isJobActionInProgress(jobId: string): boolean {
    return actionInProgressJobId.value === jobId
  }

  // ---------------------------------------------------------------------------
  // Actions: Reset
  // ---------------------------------------------------------------------------

  function $reset() {
    viewMode.value = 'table'
    compactMode.value = false
    showFilters.value = true
    clearSelection()
    cancelBulkAction()
    closeAllModals()
    cancelActionConfirmation()
    actionInProgressJobId.value = null
  }

  return {
    // State: View Preferences
    viewMode,
    compactMode,
    showFilters,

    // State: Selection
    selectedJobId,
    selectedJob,

    // State: Bulk Action
    bulkAction,

    // State: Modals
    modals,

    // State: Action Confirmation
    actionConfirmation,

    // State: Action In Progress
    actionInProgressJobId,

    // Computed
    hasSelection,
    bulkSelectionCount,
    hasBulkSelection,
    isBulkActionActive,
    hasOpenModal,
    isActionInProgress,
    isPendingConfirmation,

    // Actions: View Preferences
    setViewMode,
    toggleCompactMode,
    toggleFilters,

    // Actions: Selection
    selectJob,
    clearSelection,
    isJobSelected,

    // Actions: Bulk Selection
    startBulkAction,
    toggleBulkSelect,
    selectAllJobs,
    clearBulkSelection,
    cancelBulkAction,
    isBulkSelected,
    getBulkSelectedIds,

    // Actions: Modals
    openModal,
    closeModal,
    closeAllModals,

    // Actions: Action Confirmation
    requestActionConfirmation,
    confirmAction,
    cancelActionConfirmation,

    // Actions: Action In Progress
    setActionInProgress,
    isJobActionInProgress,

    // Reset
    $reset,
  }
})
