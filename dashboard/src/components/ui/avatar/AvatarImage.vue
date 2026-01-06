<script setup lang="ts">
import { cn } from '@/lib/utils'

interface Props {
  src: string
  alt?: string
  class?: string
}

const props = withDefaults(defineProps<Props>(), {
  alt: '',
  class: '',
})
</script>

<template>
  <img
    :src="src"
    :alt="alt"
    :class="cn('aspect-square h-full w-full', props.class)"
  >
</template>
