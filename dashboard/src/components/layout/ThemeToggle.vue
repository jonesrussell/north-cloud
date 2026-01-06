<script setup lang="ts">
import { computed } from 'vue'
import { Sun, Moon, Monitor } from 'lucide-vue-next'
import { useTheme, type Theme } from '@/composables/useTheme'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from '@/components/ui/dropdown-menu'

const { theme, isDark, setTheme } = useTheme()

const currentIcon = computed(() => {
  if (theme.value === 'system') return Monitor
  return isDark.value ? Moon : Sun
})

const themes: { value: Theme; label: string; icon: typeof Sun }[] = [
  { value: 'light', label: 'Light', icon: Sun },
  { value: 'dark', label: 'Dark', icon: Moon },
  { value: 'system', label: 'System', icon: Monitor },
]
</script>

<template>
  <DropdownMenu>
    <DropdownMenuTrigger>
      <Button
        variant="ghost"
        size="icon"
        class="h-9 w-9"
      >
        <component
          :is="currentIcon"
          class="h-4 w-4"
        />
        <span class="sr-only">Toggle theme</span>
      </Button>
    </DropdownMenuTrigger>
    <DropdownMenuContent align="end">
      <DropdownMenuItem
        v-for="t in themes"
        :key="t.value"
        :class="{ 'bg-accent': theme === t.value }"
        @select="setTheme(t.value)"
      >
        <component
          :is="t.icon"
          class="mr-2 h-4 w-4"
        />
        {{ t.label }}
      </DropdownMenuItem>
    </DropdownMenuContent>
  </DropdownMenu>
</template>
