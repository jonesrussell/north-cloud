<script setup lang="ts">
import { cn } from '@/lib/utils'

interface Props {
  class?: string
}

const props = withDefaults(defineProps<Props>(), {
  class: '',
})
</script>

<template>
  <div :class="cn('px-2 py-1.5 text-sm font-semibold', props.class)">
    <slot />
  </div>
</template>
