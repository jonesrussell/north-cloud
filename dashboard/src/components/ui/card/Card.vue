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
  <div
    :class="cn(
      'rounded-sm border bg-card text-card-foreground shadow-sm',
      'dark:shadow-none dark:border-t-[hsl(220_13%_22%)]',
      props.class
    )"
  >
    <slot />
  </div>
</template>
