<template>
  <!-- Modal backdrop -->
  <TransitionRoot :show="isOpen" as="template" @after-leave="searchQuery = ''">
    <Dialog as="div" class="relative z-50" @close="close">
      <!-- Backdrop -->
      <TransitionChild
        as="template"
        enter="ease-out duration-300"
        enter-from="opacity-0"
        enter-to="opacity-100"
        leave="ease-in duration-200"
        leave-from="opacity-100"
        leave-to="opacity-0"
      >
        <div class="fixed inset-0 bg-gray-500 bg-opacity-75 transition-opacity" />
      </TransitionChild>

      <!-- Modal panel -->
      <div class="fixed inset-0 z-10 overflow-y-auto p-4 sm:p-6 md:p-20">
        <TransitionChild
          as="template"
          enter="ease-out duration-300"
          enter-from="opacity-0 scale-95"
          enter-to="opacity-100 scale-100"
          leave="ease-in duration-200"
          leave-from="opacity-100 scale-100"
          leave-to="opacity-0 scale-95"
        >
          <DialogPanel
            class="mx-auto max-w-2xl transform divide-y divide-gray-100 overflow-hidden rounded-xl bg-white shadow-2xl ring-1 ring-black ring-opacity-5 transition-all"
          >
            <!-- Combobox for search -->
            <Combobox @update:modelValue="handleSelect">
              <!-- Search input -->
              <div class="relative">
                <MagnifyingGlassIcon
                  class="pointer-events-none absolute left-4 top-3.5 h-5 w-5 text-gray-400"
                  aria-hidden="true"
                />
                <ComboboxInput
                  class="h-12 w-full border-0 bg-transparent pl-11 pr-4 text-gray-900 placeholder:text-gray-400 focus:ring-0 sm:text-sm"
                  placeholder="Search pages... (Cmd+K)"
                  @change="searchQuery = $event.target.value"
                  :value="searchQuery"
                />
                <kbd
                  class="pointer-events-none absolute right-4 top-3 hidden sm:block px-2 py-1 text-xs font-semibold text-gray-500 bg-gray-100 rounded border border-gray-200"
                >
                  ESC
                </kbd>
              </div>

              <!-- Results -->
              <ComboboxOptions
                v-if="searchResults.length > 0"
                static
                class="max-h-96 scroll-py-2 overflow-y-auto py-2 text-sm text-gray-800"
              >
                <ComboboxOption
                  v-for="item in searchResults"
                  :key="item.path"
                  :value="item.path"
                  v-slot="{ active }"
                >
                  <div
                    :class="[
                      'cursor-pointer select-none px-4 py-2 flex items-center',
                      active ? 'bg-blue-600 text-white' : 'text-gray-900',
                    ]"
                  >
                    <!-- Icon -->
                    <component
                      :is="item.icon"
                      :class="[
                        'h-5 w-5 mr-3 flex-shrink-0',
                        active ? 'text-white' : 'text-gray-400',
                      ]"
                    />

                    <!-- Content -->
                    <div class="flex-1 min-w-0">
                      <div class="font-medium truncate">
                        {{ item.label }}
                      </div>
                      <div
                        v-if="item.description"
                        :class="[
                          'text-xs truncate',
                          active ? 'text-blue-100' : 'text-gray-500',
                        ]"
                      >
                        {{ item.description }}
                      </div>
                    </div>

                    <!-- Path badge -->
                    <div
                      :class="[
                        'ml-3 text-xs font-mono px-2 py-1 rounded',
                        active
                          ? 'bg-blue-700 text-blue-100'
                          : 'bg-gray-100 text-gray-600',
                      ]"
                    >
                      {{ item.path }}
                    </div>
                  </div>
                </ComboboxOption>
              </ComboboxOptions>

              <!-- Empty state -->
              <div
                v-if="searchQuery && searchResults.length === 0"
                class="px-6 py-14 text-center text-sm sm:px-14"
              >
                <MagnifyingGlassIcon
                  class="mx-auto h-6 w-6 text-gray-400"
                  aria-hidden="true"
                />
                <p class="mt-4 font-semibold text-gray-900">No results found</p>
                <p class="mt-2 text-gray-500">
                  No pages found for "{{ searchQuery }}". Try a different search.
                </p>
              </div>

              <!-- Help text -->
              <div
                v-if="!searchQuery && searchResults.length > 0"
                class="border-t border-gray-100 px-6 py-3 text-xs text-gray-500"
              >
                <div class="flex items-center justify-between">
                  <span>Use ↑↓ to navigate, ↵ to select, ESC to close</span>
                  <kbd class="px-2 py-1 bg-gray-100 rounded border border-gray-200 font-mono">
                    Cmd+K
                  </kbd>
                </div>
              </div>
            </Combobox>
          </DialogPanel>
        </TransitionChild>
      </div>
    </Dialog>
  </TransitionRoot>
</template>

<script setup lang="ts">
import {
  TransitionRoot,
  TransitionChild,
  Dialog,
  DialogPanel,
  Combobox,
  ComboboxInput,
  ComboboxOptions,
  ComboboxOption,
} from '@headlessui/vue'
import { MagnifyingGlassIcon } from '@heroicons/vue/24/outline'
import { useCommandPalette } from '@/composables/useCommandPalette'

const { isOpen, searchQuery, searchResults, close, navigateTo } = useCommandPalette()

const handleSelect = (path: string) => {
  navigateTo(path)
}
</script>
