<script setup lang="ts">
import { ArrowUp, ArrowDown, ArrowUpDown } from 'lucide-vue-next'

defineProps<{
  label: string
  sortKey: string
  currentSortBy: string
  currentSortOrder: 'asc' | 'desc'
}>()

defineEmits<{
  (e: 'sort'): void
}>()

function getSortIcon(sortKey: string, currentSortBy: string, currentSortOrder: 'asc' | 'desc') {
  if (currentSortBy !== sortKey) return ArrowUpDown
  return currentSortOrder === 'asc' ? ArrowUp : ArrowDown
}
</script>

<template>
  <th
    class="px-4 py-3 text-left text-sm font-medium text-muted-foreground cursor-pointer hover:text-foreground transition-colors"
    @click="$emit('sort')"
  >
    <div class="flex items-center gap-1">
      {{ label }}
      <component
        :is="getSortIcon(sortKey, currentSortBy, currentSortOrder)"
        :class="[
          'h-4 w-4',
          currentSortBy === sortKey ? 'text-foreground' : 'text-muted-foreground/50'
        ]"
      />
    </div>
  </th>
</template>
