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
  <li :class="cn('inline-flex items-center gap-1.5', props.class)">
    <slot />
  </li>
</template>
