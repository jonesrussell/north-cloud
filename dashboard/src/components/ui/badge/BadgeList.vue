<script setup lang="ts">
import { computed } from 'vue'
import Badge from './Badge.vue'
import Tooltip from '@/components/ui/tooltip/Tooltip.vue'

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
const hiddenItems = computed(() => props.items.slice(props.maxVisible))
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
    <Tooltip
      v-if="hasOverflow"
      :content="hiddenItems.join(', ')"
      side="top"
    >
      <Badge
        :variant="props.variant"
        :class="[props.badgeClass, 'cursor-default']"
      >
        +{{ overflowCount }}
      </Badge>
    </Tooltip>
  </div>
</template>
