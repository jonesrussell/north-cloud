<script setup lang="ts">
import { Plus, BarChart3, Eye, Settings } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

interface QuickAction {
  label: string
  path: string
  icon: 'plus' | 'chart' | 'view' | 'settings'
  variant?: 'default' | 'outline' | 'secondary'
}

interface Props {
  actions: QuickAction[]
  title?: string
}

const props = withDefaults(defineProps<Props>(), {
  title: 'Quick Actions',
})

const getIcon = (icon: string) => {
  switch (icon) {
    case 'plus':
      return Plus
    case 'chart':
      return BarChart3
    case 'view':
      return Eye
    case 'settings':
      return Settings
    default:
      return Plus
  }
}
</script>

<template>
  <Card>
    <CardHeader class="pb-3">
      <CardTitle class="text-sm font-medium">
        {{ title }}
      </CardTitle>
    </CardHeader>
    <CardContent class="space-y-2">
      <router-link
        v-for="action in actions"
        :key="action.path"
        :to="action.path"
        class="block"
      >
        <Button
          :variant="action.variant || 'outline'"
          class="w-full justify-start"
        >
          <component
            :is="getIcon(action.icon)"
            class="mr-2 h-4 w-4"
          />
          {{ action.label }}
        </Button>
      </router-link>
    </CardContent>
  </Card>
</template>
