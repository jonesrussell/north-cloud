<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Loader2, Save } from 'lucide-vue-next'
import { sourcesApi } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'

const route = useRoute()
const router = useRouter()

const sourceId = computed(() => route.params.id as string | undefined)
const isEdit = computed(() => !!sourceId.value)

const loading = ref(false)
const saving = ref(false)
const error = ref<string | null>(null)

const form = ref({
  name: '',
  url: '',
  enabled: true,
})

const loadSource = async () => {
  if (!sourceId.value) return
  try {
    loading.value = true
    const response = await sourcesApi.get(sourceId.value)
    const source = response.data
    form.value = {
      name: source.name || '',
      url: source.url || '',
      enabled: source.enabled ?? true,
    }
  } catch (err) {
    error.value = 'Unable to load source.'
  } finally {
    loading.value = false
  }
}

const saveSource = async () => {
  error.value = null
  if (!form.value.name || !form.value.url) {
    error.value = 'Name and URL are required.'
    return
  }

  try {
    saving.value = true
    if (isEdit.value && sourceId.value) {
      await sourcesApi.update(sourceId.value, form.value)
    } else {
      await sourcesApi.create(form.value)
    }
    router.push('/scheduling/sources')
  } catch (err: unknown) {
    const e = err as { response?: { data?: { error?: string } } }
    error.value = e.response?.data?.error || 'Failed to save source.'
  } finally {
    saving.value = false
  }
}

onMounted(loadSource)
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center gap-4">
      <Button
        variant="ghost"
        size="icon"
        @click="router.push('/scheduling/sources')"
      >
        <ArrowLeft class="h-5 w-5" />
      </Button>
      <div>
        <h1 class="text-3xl font-bold tracking-tight">
          {{ isEdit ? 'Edit Source' : 'New Source' }}
        </h1>
        <p class="text-muted-foreground">
          {{ isEdit ? 'Update source configuration' : 'Add a new content source' }}
        </p>
      </div>
    </div>

    <div
      v-if="loading"
      class="flex items-center justify-center py-12"
    >
      <Loader2 class="h-8 w-8 animate-spin text-muted-foreground" />
    </div>

    <Card
      v-else
      class="max-w-2xl"
    >
      <CardHeader>
        <CardTitle>Source Details</CardTitle>
        <CardDescription>Configure the basic information for this source</CardDescription>
      </CardHeader>
      <CardContent>
        <form
          class="space-y-4"
          @submit.prevent="saveSource"
        >
          <div>
            <label class="block text-sm font-medium mb-2">Name</label>
            <Input
              v-model="form.name"
              placeholder="My News Source"
            />
          </div>

          <div>
            <label class="block text-sm font-medium mb-2">URL</label>
            <Input
              v-model="form.url"
              type="url"
              placeholder="https://example.com"
            />
          </div>

          <div class="flex items-center gap-2">
            <input
              id="enabled"
              v-model="form.enabled"
              type="checkbox"
              class="h-4 w-4"
            >
            <label
              for="enabled"
              class="text-sm"
            >Enabled</label>
          </div>

          <div
            v-if="error"
            class="p-3 text-sm text-destructive bg-destructive/10 rounded-md"
          >
            {{ error }}
          </div>

          <div class="flex justify-end gap-3 pt-4">
            <Button
              type="button"
              variant="outline"
              @click="router.push('/scheduling/sources')"
            >
              Cancel
            </Button>
            <Button
              type="submit"
              :disabled="saving"
            >
              <Loader2
                v-if="saving"
                class="mr-2 h-4 w-4 animate-spin"
              />
              <Save
                v-else
                class="mr-2 h-4 w-4"
              />
              {{ saving ? 'Saving...' : 'Save Source' }}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  </div>
</template>
