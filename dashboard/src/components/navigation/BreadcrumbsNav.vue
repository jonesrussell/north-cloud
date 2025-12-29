<template>
  <nav
    v-if="breadcrumbs.length > 0"
    aria-label="Breadcrumb"
    class="flex items-center space-x-2 text-sm"
  >
    <template
      v-for="(crumb, index) in displayBreadcrumbs"
      :key="crumb.path"
    >
      <!-- Chevron separator -->
      <ChevronRightIcon
        v-if="index > 0"
        class="h-4 w-4 text-gray-400 flex-shrink-0"
      />

      <!-- Breadcrumb item -->
      <div class="flex items-center">
        <component
          :is="crumb.icon"
          v-if="crumb.icon"
          class="h-4 w-4 mr-1.5 text-gray-500"
        />

        <!-- Last item (current page) - not clickable -->
        <span
          v-if="index === displayBreadcrumbs.length - 1"
          class="font-medium text-gray-900"
        >
          {{ crumb.label }}
        </span>

        <!-- Clickable breadcrumb link -->
        <router-link
          v-else
          :to="crumb.path"
          class="text-gray-600 hover:text-gray-900 transition-colors"
        >
          {{ crumb.label }}
        </router-link>
      </div>
    </template>

    <!-- Show collapsed indicator if we've hidden some breadcrumbs -->
    <template v-if="breadcrumbs.length > maxBreadcrumbs">
      <ChevronRightIcon class="h-4 w-4 text-gray-400 flex-shrink-0" />
      <span class="text-gray-500 text-xs">
        +{{ breadcrumbs.length - maxBreadcrumbs }} more
      </span>
    </template>
  </nav>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { ChevronRightIcon, HomeIcon } from '@heroicons/vue/24/outline'
import type { Component } from 'vue'

interface Breadcrumb {
  label: string
  path: string
  icon?: Component
}

const props = withDefaults(defineProps<{
  maxBreadcrumbs?: number
}>(), {
  maxBreadcrumbs: 4,
})

const route = useRoute()

/**
 * Generate breadcrumbs from route meta or auto-generate from route hierarchy
 */
const breadcrumbs = computed<Breadcrumb[]>(() => {
  const crumbs: Breadcrumb[] = []

  // If route has explicit breadcrumbs in meta, use those
  if (route.meta.breadcrumbs && Array.isArray(route.meta.breadcrumbs)) {
    return route.meta.breadcrumbs as Breadcrumb[]
  }

  // Otherwise, auto-generate breadcrumbs from path segments
  const pathSegments = route.path.split('/').filter((segment) => segment !== '')

  // Always start with Dashboard (home)
  if (route.path !== '/') {
    crumbs.push({
      label: 'Dashboard',
      path: '/',
      icon: HomeIcon,
    })
  }

  // Build breadcrumbs from path segments
  let currentPath = ''
  pathSegments.forEach((segment) => {
    currentPath += `/${segment}`

    // Skip numeric segments (likely IDs like /sources/123/edit)
    if (/^\d+$/.test(segment)) {
      return
    }

    // Get friendly label from route meta or format segment
    let label = segment
      .split('-')
      .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
      .join(' ')

    // Try to get title from route meta if this is the current route
    if (currentPath === route.path && route.meta.title) {
      label = route.meta.title as string
    }

    crumbs.push({
      label,
      path: currentPath,
    })
  })

  return crumbs
})

/**
 * Display breadcrumbs, collapsing if too many
 * Shows first, last, and middle breadcrumbs up to maxBreadcrumbs
 */
const displayBreadcrumbs = computed<Breadcrumb[]>(() => {
  if (breadcrumbs.value.length <= props.maxBreadcrumbs) {
    return breadcrumbs.value
  }

  // Show first 2, last 1, and fill middle
  const first = breadcrumbs.value.slice(0, 2)
  const last = breadcrumbs.value.slice(-1)
  const middleCount = props.maxBreadcrumbs - 3
  const middle = breadcrumbs.value.slice(2, 2 + middleCount)

  return [...first, ...middle, ...last]
})
</script>
