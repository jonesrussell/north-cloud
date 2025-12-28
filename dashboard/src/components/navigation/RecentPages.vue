<template>
  <Menu as="div" class="relative inline-block text-left">
    <MenuButton
      class="inline-flex items-center rounded-md px-3 py-2 text-sm font-medium text-gray-700 hover:bg-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
    >
      <ClockIcon class="h-5 w-5 mr-2 text-gray-500" />
      Recent
      <ChevronDownIcon class="ml-2 h-4 w-4 text-gray-500" />
    </MenuButton>

    <transition
      enter-active-class="transition ease-out duration-100"
      enter-from-class="transform opacity-0 scale-95"
      enter-to-class="transform opacity-100 scale-100"
      leave-active-class="transition ease-in duration-75"
      leave-from-class="transform opacity-100 scale-100"
      leave-to-class="transform opacity-0 scale-95"
    >
      <MenuItems
        class="absolute right-0 z-10 mt-2 w-80 origin-top-right rounded-md bg-white shadow-lg ring-1 ring-black ring-opacity-5 focus:outline-none"
      >
        <div class="py-1">
          <!-- Header -->
          <div class="px-4 py-2 border-b border-gray-200">
            <div class="flex items-center justify-between">
              <h3 class="text-sm font-semibold text-gray-900">Recent Pages</h3>
              <button
                v-if="recentPages.length > 0"
                @click="handleClear"
                class="text-xs text-gray-500 hover:text-gray-700"
              >
                Clear all
              </button>
            </div>
          </div>

          <!-- Recent pages list -->
          <div v-if="recentPages.length > 0" class="max-h-96 overflow-y-auto">
            <MenuItem
              v-for="page in recentPages"
              :key="page.path"
              v-slot="{ active }"
            >
              <div
                @click="navigateToRecentPage(page.path)"
                :class="[
                  active ? 'bg-gray-100' : '',
                  'group flex items-center px-4 py-3 text-sm cursor-pointer hover:bg-gray-50',
                ]"
              >
                <!-- Icon -->
                <component
                  :is="page.icon || DocumentTextIcon"
                  class="h-5 w-5 mr-3 text-gray-400 flex-shrink-0"
                />

                <!-- Content -->
                <div class="flex-1 min-w-0">
                  <div class="font-medium text-gray-900 truncate">
                    {{ page.title }}
                  </div>
                  <div class="text-xs text-gray-500 truncate">
                    {{ page.path }}
                  </div>
                </div>

                <!-- Time -->
                <div class="ml-3 text-xs text-gray-400 flex-shrink-0">
                  {{ page.timeAgo }}
                </div>

                <!-- Remove button (on hover) -->
                <button
                  @click.stop="removeRecentPage(page.path)"
                  class="ml-2 opacity-0 group-hover:opacity-100 transition-opacity"
                >
                  <XMarkIcon class="h-4 w-4 text-gray-400 hover:text-gray-600" />
                </button>
              </div>
            </MenuItem>
          </div>

          <!-- Empty state -->
          <div
            v-else
            class="px-4 py-8 text-center text-sm text-gray-500"
          >
            <ClockIcon class="mx-auto h-8 w-8 text-gray-300 mb-2" />
            <p>No recent pages</p>
            <p class="text-xs mt-1">
              Pages you visit will appear here
            </p>
          </div>
        </div>
      </MenuItems>
    </transition>
  </Menu>
</template>

<script setup lang="ts">
import { Menu, MenuButton, MenuItems, MenuItem } from '@headlessui/vue'
import {
  ClockIcon,
  ChevronDownIcon,
  XMarkIcon,
  DocumentTextIcon,
} from '@heroicons/vue/24/outline'
import { useRecentPages } from '@/composables/useRecentPages'

const {
  recentPages,
  clearRecentPages,
  removeRecentPage,
  navigateToRecentPage,
} = useRecentPages()

const handleClear = () => {
  if (confirm('Clear all recent pages?')) {
    clearRecentPages()
  }
}
</script>
