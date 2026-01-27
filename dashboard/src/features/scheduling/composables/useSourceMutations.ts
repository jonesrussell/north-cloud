/**
 * Sources Mutation Composables
 *
 * TanStack Query mutation hooks for source CRUD operations.
 */

import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { useToast } from '@/composables/useToast'
import {
  sourcesKeys,
  createSource,
  updateSource,
  deleteSource,
  type CreateSourceRequest,
  type UpdateSourceRequest,
  type Source,
} from '../api/sources'

// ============================================================================
// Create Source Mutation
// ============================================================================

export function useCreateSourceMutation() {
  const queryClient = useQueryClient()
  const toast = useToast()

  return useMutation({
    mutationFn: (data: CreateSourceRequest) => createSource(data),
    onSuccess: (newSource) => {
      queryClient.invalidateQueries({ queryKey: sourcesKeys.lists() })
      toast.success('Source created', {
        description: `${newSource.name} has been created successfully.`,
      })
    },
    onError: (error) => {
      toast.error('Failed to create source', {
        description: error.message,
      })
    },
  })
}

// ============================================================================
// Update Source Mutation
// ============================================================================

export function useUpdateSourceMutation() {
  const queryClient = useQueryClient()
  const toast = useToast()

  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateSourceRequest }) =>
      updateSource(id, data),
    onSuccess: (updatedSource) => {
      queryClient.invalidateQueries({ queryKey: sourcesKeys.lists() })
      queryClient.invalidateQueries({ queryKey: sourcesKeys.detail(updatedSource.id) })
      toast.success('Source updated', {
        description: `${updatedSource.name} has been updated.`,
      })
    },
    onError: (error) => {
      toast.error('Failed to update source', {
        description: error.message,
      })
    },
  })
}

// ============================================================================
// Delete Source Mutation
// ============================================================================

export function useDeleteSourceMutation() {
  const queryClient = useQueryClient()
  const toast = useToast()

  return useMutation({
    mutationFn: (id: string) => deleteSource(id),
    onSuccess: (_, deletedId) => {
      queryClient.invalidateQueries({ queryKey: sourcesKeys.lists() })
      queryClient.removeQueries({ queryKey: sourcesKeys.detail(deletedId) })
      toast.success('Source deleted')
    },
    onError: (error) => {
      toast.error('Failed to delete source', {
        description: error.message,
      })
    },
  })
}

// ============================================================================
// Toggle Source Enabled Mutation
// ============================================================================

export function useToggleSourceMutation() {
  const queryClient = useQueryClient()
  const toast = useToast()

  return useMutation({
    mutationFn: ({ id, is_enabled }: { id: string; is_enabled: boolean }) =>
      updateSource(id, { is_enabled }),
    onMutate: async ({ id, is_enabled }) => {
      // Cancel outgoing refetches
      await queryClient.cancelQueries({ queryKey: sourcesKeys.lists() })

      // Snapshot previous value
      const previousSources = queryClient.getQueryData(sourcesKeys.lists())

      // Optimistically update
      queryClient.setQueryData(sourcesKeys.lists(), (old: { sources: Source[] } | undefined) => {
        if (!old) return old
        return {
          ...old,
          sources: old.sources.map((s) =>
            s.id === id ? { ...s, is_enabled } : s
          ),
        }
      })

      return { previousSources }
    },
    onError: (error, _, context) => {
      // Rollback on error
      if (context?.previousSources) {
        queryClient.setQueryData(sourcesKeys.lists(), context.previousSources)
      }
      toast.error('Failed to update source', {
        description: error.message,
      })
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: sourcesKeys.lists() })
    },
  })
}

// ============================================================================
// Combined Mutations Hook
// ============================================================================

export function useSourceMutations() {
  return {
    create: useCreateSourceMutation(),
    update: useUpdateSourceMutation(),
    delete: useDeleteSourceMutation(),
    toggle: useToggleSourceMutation(),
  }
}
