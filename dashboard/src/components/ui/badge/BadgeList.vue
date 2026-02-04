<script setup lang="ts">
import { computed } from 'vue'
import Badge from './Badge.vue'

type BadgeVariant = 'default' | 'secondary' | 'destructive' | 'outline' | 'success' | 'warning' | 'pending'

interface Props {
  items: string[]
  maxVisible?: number
  variant?: BadgeVariant
  badgeClass?: string
}

const props = withDefaults(defineProps<Props>(), {
  maxVisible: 3,
  variant: 'outline',
  badgeClass: 'text-xs',
})

const visibleItems = computed(() => props.items.slice(0, props.maxVisible))
const overflowCount = computed(() => Math.max(0, props.items.length - props.maxVisible))
const hasOverflow = computed(() => overflowCount.value > 0)
</script>

<template>
  <div class="flex flex-wrap gap-1">
    <Badge
      v-for="item in visibleItems"
      :key="item"
      :variant="props.variant"
      :class="props.badgeClass"
    >
      {{ item }}
    </Badge>
    <Badge
      v-if="hasOverflow"
      :variant="props.variant"
      :class="props.badgeClass"
    >
      +{{ overflowCount }}
    </Badge>
  </div>
</template>
