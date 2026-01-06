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
  <h3 :class="cn('text-lg font-semibold text-foreground', props.class)">
    <slot />
  </h3>
</template>
