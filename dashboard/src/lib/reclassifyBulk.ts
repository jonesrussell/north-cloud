import { classifierApi } from '@/api/client'
import type { AxiosError } from 'axios'

const defaultConcurrency = 3
const maxConcurrency = 5

export interface ReclassifyBulkOptions {
  concurrency?: number
  onProgress?: (done: number, total: number) => void
}

export interface ReclassifyBulkError {
  id: string
  error: string
}

export interface ReclassifyBulkResult {
  succeeded: number
  failed: number
  errors: ReclassifyBulkError[]
}

function getErrorMessage(err: unknown): string {
  const axiosErr = err as AxiosError
  if (axiosErr?.response?.status === 404) {
    return 'Not found'
  }
  const data = axiosErr?.response?.data as { error?: string } | undefined
  if (typeof data?.error === 'string') {
    return data.error
  }
  return axiosErr?.message ?? String(err)
}

async function processWithConcurrency(
  ids: string[],
  limit: number,
  onProgress?: (done: number, total: number) => void
): Promise<ReclassifyBulkResult> {
  const errors: ReclassifyBulkError[] = []
  let succeeded = 0
  let done = 0
  const total = ids.length

  async function processOne(id: string): Promise<void> {
    try {
      await classifierApi.classify.reclassify(id)
      succeeded++
    } catch (err) {
      errors.push({ id, error: getErrorMessage(err) })
    } finally {
      done++
      onProgress?.(done, total)
    }
  }

  let index = 0
  async function worker(): Promise<void> {
    while (index < ids.length) {
      const i = index++
      if (i >= ids.length) break
      await processOne(ids[i])
    }
  }

  const workers = Array.from({ length: limit }, () => worker())
  await Promise.all(workers)

  return {
    succeeded,
    failed: errors.length,
    errors,
  }
}

/**
 * Reclassify multiple documents with concurrency limit.
 * 404 is treated as soft failure; batch continues.
 */
export async function reclassifyBulk(
  ids: string[],
  options?: ReclassifyBulkOptions
): Promise<ReclassifyBulkResult> {
  const concurrency = Math.min(
    options?.concurrency ?? defaultConcurrency,
    maxConcurrency
  )
  return processWithConcurrency(ids, concurrency, options?.onProgress)
}
