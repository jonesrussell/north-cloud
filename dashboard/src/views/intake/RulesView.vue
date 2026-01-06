<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Loader2, Filter, Plus } from 'lucide-vue-next'
import { classifierApi } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'

interface Rule {
  id: string
  name: string
  type: string
  pattern: string
  enabled: boolean
  created_at: string
}

const loading = ref(true)
const error = ref<string | null>(null)
const rules = ref<Rule[]>([])

const loadRules = async () => {
  try {
    loading.value = true
    const response = await classifierApi.rules.list()
    rules.value = response.data?.rules || response.data || []
  } catch (err) {
    error.value = 'Unable to load classification rules.'
  } finally {
    loading.value = false
  }
}

onMounted(loadRules)
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-3xl font-bold tracking-tight">Classification Rules</h1>
        <p class="text-muted-foreground">Configure rules for content classification</p>
      </div>
      <Button>
        <Plus class="mr-2 h-4 w-4" />
        Add Rule
      </Button>
    </div>

    <div v-if="loading" class="flex items-center justify-center py-12">
      <Loader2 class="h-8 w-8 animate-spin text-muted-foreground" />
    </div>

    <Card v-else-if="error" class="border-destructive">
      <CardContent class="pt-6">
        <p class="text-destructive">{{ error }}</p>
      </CardContent>
    </Card>

    <Card v-else-if="rules.length === 0">
      <CardContent class="flex flex-col items-center justify-center py-12">
        <Filter class="h-12 w-12 text-muted-foreground mb-4" />
        <h3 class="text-lg font-medium mb-2">No classification rules</h3>
        <p class="text-muted-foreground mb-4">Add rules to classify content automatically.</p>
        <Button>
          <Plus class="mr-2 h-4 w-4" />
          Add Rule
        </Button>
      </CardContent>
    </Card>

    <Card v-else>
      <CardContent class="p-0">
        <table class="w-full">
          <thead class="border-b bg-muted/50">
            <tr>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">Name</th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">Type</th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">Pattern</th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">Status</th>
            </tr>
          </thead>
          <tbody class="divide-y">
            <tr v-for="rule in rules" :key="rule.id" class="hover:bg-muted/50">
              <td class="px-6 py-4 text-sm font-medium">{{ rule.name }}</td>
              <td class="px-6 py-4 text-sm text-muted-foreground">{{ rule.type }}</td>
              <td class="px-6 py-4 text-sm font-mono text-muted-foreground">{{ rule.pattern }}</td>
              <td class="px-6 py-4">
                <Badge :variant="rule.enabled ? 'success' : 'secondary'">
                  {{ rule.enabled ? 'Enabled' : 'Disabled' }}
                </Badge>
              </td>
            </tr>
          </tbody>
        </table>
      </CardContent>
    </Card>
  </div>
</template>
