<template>
  <div
    v-if="isOpen"
    class="fixed inset-0 z-50 overflow-y-auto"
    @click.self="close"
  >
    <div class="flex items-center justify-center min-h-screen px-4 pt-4 pb-20 text-center sm:p-0">
      <!-- Backdrop -->
      <div
        class="fixed inset-0 transition-opacity bg-gray-500 bg-opacity-75"
        @click="close"
      />

      <!-- Modal panel -->
      <div class="relative inline-block w-full max-w-2xl px-4 pt-5 pb-4 overflow-hidden text-left align-bottom transition-all transform bg-white rounded-lg shadow-xl sm:my-8 sm:align-middle sm:p-6">
        <!-- Header -->
        <div class="flex items-center justify-between mb-6">
          <div>
            <h2 class="text-2xl font-bold text-gray-900">
              Import Sources from Excel
            </h2>
            <p class="mt-1 text-sm text-gray-600">
              Upload an Excel file (.xlsx) to import sources in bulk
            </p>
          </div>
          <button
            class="text-gray-400 hover:text-gray-500 focus:outline-none"
            :disabled="uploading"
            @click="close"
          >
            <span class="sr-only">Close</span>
            <XMarkIcon class="w-6 h-6" />
          </button>
        </div>

        <!-- Upload State -->
        <div v-if="!uploading && !result && !error">
          <div class="space-y-4">
            <!-- Template Info -->
            <div class="bg-blue-50 border border-blue-200 rounded-lg p-4">
              <div class="flex">
                <DocumentArrowDownIcon class="w-5 h-5 text-blue-600 mt-0.5" />
                <div class="ml-3">
                  <p class="text-sm text-blue-800 font-medium">
                    Excel Format Requirements
                  </p>
                  <p class="mt-1 text-xs text-blue-700">
                    Your Excel file should include columns: name, url, enabled, and optional selector fields.
                    Sources are matched by name (existing sources will be updated, new ones created).
                  </p>
                </div>
              </div>
            </div>

            <!-- File Input -->
            <div>
              <label class="block text-sm font-medium text-gray-700 mb-2">
                Excel File
              </label>
              <div
                class="flex justify-center px-6 pt-5 pb-6 border-2 border-dashed rounded-lg transition-colors"
                :class="dragOver ? 'border-blue-400 bg-blue-50' : 'border-gray-300 hover:border-gray-400'"
                @dragover.prevent="dragOver = true"
                @dragleave.prevent="dragOver = false"
                @drop.prevent="handleDrop"
              >
                <div class="space-y-1 text-center">
                  <DocumentArrowUpIcon class="mx-auto h-12 w-12 text-gray-400" />
                  <div class="flex text-sm text-gray-600">
                    <label
                      for="file-upload"
                      class="relative cursor-pointer bg-white rounded-md font-medium text-blue-600 hover:text-blue-500 focus-within:outline-none focus-within:ring-2 focus-within:ring-offset-2 focus-within:ring-blue-500"
                    >
                      <span>Upload a file</span>
                      <input
                        id="file-upload"
                        ref="fileInput"
                        type="file"
                        accept=".xlsx"
                        class="sr-only"
                        @change="handleFileChange"
                      >
                    </label>
                    <p class="pl-1">
                      or drag and drop
                    </p>
                  </div>
                  <p class="text-xs text-gray-500">
                    Excel files only (.xlsx)
                  </p>
                </div>
              </div>

              <!-- Selected File -->
              <div
                v-if="selectedFile"
                class="mt-3 flex items-center justify-between bg-gray-50 rounded-lg p-3"
              >
                <div class="flex items-center">
                  <DocumentTextIcon class="w-5 h-5 text-gray-400 mr-2" />
                  <span class="text-sm text-gray-700">{{ selectedFile.name }}</span>
                  <span class="text-xs text-gray-500 ml-2">
                    ({{ formatFileSize(selectedFile.size) }})
                  </span>
                </div>
                <button
                  type="button"
                  class="text-gray-400 hover:text-gray-500"
                  @click="clearFile"
                >
                  <XMarkIcon class="w-4 h-4" />
                </button>
              </div>
            </div>
          </div>

          <!-- Actions -->
          <div class="mt-6 flex justify-end gap-3">
            <button
              type="button"
              class="px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500"
              @click="close"
            >
              Cancel
            </button>
            <button
              type="button"
              class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed"
              :disabled="!selectedFile"
              @click="handleUpload"
            >
              Import
            </button>
          </div>
        </div>

        <!-- Loading State -->
        <div
          v-else-if="uploading"
          class="flex flex-col items-center justify-center py-12"
        >
          <div class="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600" />
          <p class="mt-4 text-sm text-gray-600">
            Importing sources...
          </p>
        </div>

        <!-- Results State -->
        <div
          v-else-if="result"
          class="space-y-6"
        >
          <!-- Summary Stats -->
          <div class="grid grid-cols-3 gap-4">
            <div class="bg-green-50 rounded-lg p-4">
              <div class="flex items-center">
                <PlusCircleIcon class="w-8 h-8 text-green-600" />
                <div class="ml-3">
                  <p class="text-sm font-medium text-green-900">
                    Created
                  </p>
                  <p class="text-2xl font-bold text-green-600">
                    {{ result.created }}
                  </p>
                </div>
              </div>
            </div>

            <div class="bg-blue-50 rounded-lg p-4">
              <div class="flex items-center">
                <ArrowPathIcon class="w-8 h-8 text-blue-600" />
                <div class="ml-3">
                  <p class="text-sm font-medium text-blue-900">
                    Updated
                  </p>
                  <p class="text-2xl font-bold text-blue-600">
                    {{ result.updated }}
                  </p>
                </div>
              </div>
            </div>

            <div class="bg-red-50 rounded-lg p-4">
              <div class="flex items-center">
                <ExclamationCircleIcon class="w-8 h-8 text-red-600" />
                <div class="ml-3">
                  <p class="text-sm font-medium text-red-900">
                    Errors
                  </p>
                  <p class="text-2xl font-bold text-red-600">
                    {{ result.errors?.length || 0 }}
                  </p>
                </div>
              </div>
            </div>
          </div>

          <!-- Success Message -->
          <div
            v-if="result.created > 0 || result.updated > 0"
            class="bg-green-50 border border-green-200 rounded-lg p-4"
          >
            <div class="flex">
              <CheckCircleIcon class="w-5 h-5 text-green-600 mt-0.5" />
              <div class="ml-3">
                <p class="text-sm text-green-800">
                  Successfully imported {{ result.created + result.updated }} source(s).
                </p>
              </div>
            </div>
          </div>

          <!-- Errors Table -->
          <div
            v-if="result.errors && result.errors.length > 0"
            class="bg-red-50 border border-red-200 rounded-lg overflow-hidden"
          >
            <div class="px-4 py-3 border-b border-red-200">
              <div class="flex items-center">
                <ExclamationTriangleIcon class="w-5 h-5 text-red-600" />
                <h3 class="ml-2 text-sm font-medium text-red-800">
                  Row Errors ({{ result.errors.length }})
                </h3>
              </div>
            </div>
            <div class="max-h-48 overflow-y-auto">
              <table class="min-w-full divide-y divide-red-200">
                <thead class="bg-red-100">
                  <tr>
                    <th class="px-4 py-2 text-left text-xs font-medium text-red-800 uppercase">
                      Row
                    </th>
                    <th class="px-4 py-2 text-left text-xs font-medium text-red-800 uppercase">
                      Error
                    </th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-red-200">
                  <tr
                    v-for="rowError in result.errors"
                    :key="rowError.row"
                    class="bg-white"
                  >
                    <td class="px-4 py-2 text-sm text-red-700 font-medium">
                      {{ rowError.row }}
                    </td>
                    <td class="px-4 py-2 text-sm text-red-700">
                      {{ rowError.error }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <!-- Actions -->
          <div class="flex justify-end gap-3">
            <button
              type="button"
              class="px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500"
              @click="close"
            >
              Close
            </button>
          </div>
        </div>

        <!-- Error State -->
        <div
          v-else-if="error"
          class="space-y-6"
        >
          <div class="bg-red-50 border border-red-200 rounded-lg p-4">
            <div class="flex">
              <XCircleIcon class="w-5 h-5 text-red-600 mt-0.5" />
              <div class="ml-3">
                <h3 class="text-sm font-medium text-red-800">
                  Import Failed
                </h3>
                <p class="mt-2 text-sm text-red-700">
                  {{ error }}
                </p>
              </div>
            </div>
          </div>

          <!-- Actions -->
          <div class="flex justify-end gap-3">
            <button
              type="button"
              class="px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500"
              @click="reset"
            >
              Try Again
            </button>
            <button
              type="button"
              class="px-4 py-2 bg-gray-600 text-white rounded-md hover:bg-gray-700 focus:outline-none focus:ring-2 focus:ring-gray-500"
              @click="close"
            >
              Close
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import {
  XMarkIcon,
  DocumentArrowDownIcon,
  DocumentArrowUpIcon,
  DocumentTextIcon,
  PlusCircleIcon,
  ArrowPathIcon,
  ExclamationCircleIcon,
  ExclamationTriangleIcon,
  CheckCircleIcon,
  XCircleIcon,
} from '@heroicons/vue/24/outline'
import { sourcesApi } from '../../api/client'
import type { ImportExcelResult } from '../../types/source'
import type { ApiError } from '../../types/common'

const emit = defineEmits<{
  (e: 'imported'): void
  (e: 'close'): void
}>()

const isOpen = ref(false)
const uploading = ref(false)
const selectedFile = ref<File | null>(null)
const result = ref<ImportExcelResult | null>(null)
const error = ref<string | null>(null)
const dragOver = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)

function open() {
  isOpen.value = true
  reset()
}

function close() {
  if (uploading.value) return // Don't close while uploading

  // If we had a successful import, emit the imported event
  if (result.value && (result.value.created > 0 || result.value.updated > 0)) {
    emit('imported')
  }

  isOpen.value = false
  emit('close')
}

function reset() {
  selectedFile.value = null
  result.value = null
  error.value = null
  uploading.value = false
  dragOver.value = false
  if (fileInput.value) {
    fileInput.value.value = ''
  }
}

function handleFileChange(event: Event) {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  if (file) {
    validateAndSetFile(file)
  }
}

function handleDrop(event: DragEvent) {
  dragOver.value = false
  const file = event.dataTransfer?.files?.[0]
  if (file) {
    validateAndSetFile(file)
  }
}

function validateAndSetFile(file: File) {
  // Validate file extension
  const validExtension = file.name.toLowerCase().endsWith('.xlsx')
  // Validate MIME type (xlsx files have this specific type)
  const xlsxMimeType = 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'
  const validMimeType = file.type === xlsxMimeType || file.type === ''  // Some browsers may not set type

  if (!validExtension) {
    error.value = 'Please select an Excel file (.xlsx)'
    return
  }

  if (!validMimeType) {
    error.value = 'Invalid file type. Please select a valid Excel file (.xlsx)'
    return
  }

  selectedFile.value = file
  error.value = null
}

function clearFile() {
  selectedFile.value = null
  if (fileInput.value) {
    fileInput.value.value = ''
  }
}

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
}

async function handleUpload() {
  if (!selectedFile.value) return

  uploading.value = true
  error.value = null

  try {
    const response = await sourcesApi.importExcel(selectedFile.value)
    result.value = response.data
  } catch (err: unknown) {
    // Backend returns 400 when there are validation errors, but still includes results
    const axiosError = err as ApiError & {
      response?: { data?: ImportExcelResult & { error?: string } }
    }
    const responseData = axiosError.response?.data

    // Check if response contains valid import results (even with errors)
    if (responseData && typeof responseData.created === 'number' && typeof responseData.updated === 'number') {
      result.value = {
        created: responseData.created,
        updated: responseData.updated,
        errors: responseData.errors || [],
      }
    } else {
      error.value = responseData?.error || 'Failed to import Excel file. Please try again.'
    }
  } finally {
    uploading.value = false
  }
}

defineExpose({
  open,
  close,
})
</script>
