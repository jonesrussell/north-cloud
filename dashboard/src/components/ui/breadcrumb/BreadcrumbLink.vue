<script setup lang="ts">
import { cn } from '@/lib/utils'

interface Props {
  href?: string
  class?: string
}

const props = withDefaults(defineProps<Props>(), {
  href: '#',
  class: '',
})
</script>

<template>
  <a
    :href="href"
    :class="cn('transition-colors hover:text-foreground', props.class)"
  >
    <slot />
  </a>
</template>
