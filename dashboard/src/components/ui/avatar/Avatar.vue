<script setup lang="ts">
import { computed } from 'vue'
import { cn } from '@/lib/utils'

interface Props {
  class?: string
}

const props = withDefaults(defineProps<Props>(), {
  class: '',
})

const avatarClass = computed(() =>
  cn('relative flex h-10 w-10 shrink-0 overflow-hidden rounded-full', props.class)
)
</script>

<template>
  <span :class="avatarClass">
    <slot />
  </span>
</template>
