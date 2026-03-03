<script setup lang="ts">
import { ref, watch } from 'vue'
import { Loader2 } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import type { SocialAccount, CreateAccountRequest, UpdateAccountRequest } from '@/types/socialPublisher'

interface Props {
  open: boolean
  account?: SocialAccount | null
  saving: boolean
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'save', data: CreateAccountRequest | UpdateAccountRequest): void
}>()

const name = ref('')
const platform = ref('')
const project = ref('')
const enabled = ref(true)
const credentials = ref('')
const tokenExpiry = ref('')

const platformOptions = ['x', 'facebook', 'instagram', 'linkedin', 'mastodon'] as const

const isEdit = ref(false)

watch(() => props.open, (open) => {
  if (open && props.account) {
    isEdit.value = true
    name.value = props.account.name
    platform.value = props.account.platform
    project.value = props.account.project
    enabled.value = props.account.enabled
    credentials.value = ''
    tokenExpiry.value = props.account.token_expiry ?? ''
  } else if (open) {
    isEdit.value = false
    name.value = ''
    platform.value = 'x'
    project.value = ''
    enabled.value = true
    credentials.value = ''
    tokenExpiry.value = ''
  }
})

function handleSubmit() {
  const base: CreateAccountRequest = {
    name: name.value,
    platform: platform.value,
    project: project.value,
    enabled: enabled.value,
    token_expiry: tokenExpiry.value || undefined,
  }

  if (credentials.value.trim()) {
    try {
      base.credentials = JSON.parse(credentials.value) as Record<string, unknown>
    } catch {
      return // Invalid JSON — don't submit
    }
  }

  emit('save', base)
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="open"
      class="fixed inset-0 z-50 flex items-center justify-center"
    >
      <div
        class="fixed inset-0 bg-black/50"
        @click="emit('close')"
      />
      <div class="relative z-50 w-full max-w-lg rounded-lg border bg-background p-6 shadow-lg">
        <h2 class="mb-4 text-lg font-semibold">
          {{ isEdit ? 'Edit Account' : 'Add Account' }}
        </h2>

        <form
          class="space-y-4"
          @submit.prevent="handleSubmit"
        >
          <div>
            <label class="mb-1 block text-sm font-medium">Name</label>
            <Input
              v-model="name"
              placeholder="Account name"
              required
            />
          </div>

          <div>
            <label class="mb-1 block text-sm font-medium">Platform</label>
            <select
              v-model="platform"
              class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
            >
              <option
                v-for="p in platformOptions"
                :key="p"
                :value="p"
              >
                {{ p }}
              </option>
            </select>
          </div>

          <div>
            <label class="mb-1 block text-sm font-medium">Project</label>
            <Input
              v-model="project"
              placeholder="Project name"
              required
            />
          </div>

          <div class="flex items-center gap-2">
            <input
              id="account-enabled"
              v-model="enabled"
              type="checkbox"
              class="h-4 w-4 rounded border-gray-300"
            >
            <label
              for="account-enabled"
              class="text-sm font-medium"
            >Enabled</label>
          </div>

          <div>
            <label class="mb-1 block text-sm font-medium">
              Credentials (JSON)
              <span class="text-xs text-muted-foreground">
                {{ isEdit ? '\u2014 leave blank to keep current' : '' }}
              </span>
            </label>
            <textarea
              v-model="credentials"
              rows="4"
              :placeholder="isEdit ? 'Leave blank to keep current credentials' : '{&quot;api_key&quot;: &quot;...&quot;}'"
              class="flex w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 font-mono"
            />
          </div>

          <div>
            <label class="mb-1 block text-sm font-medium">Token Expiry (optional)</label>
            <Input
              v-model="tokenExpiry"
              type="datetime-local"
            />
          </div>

          <div class="flex justify-end gap-2 pt-2">
            <Button
              variant="outline"
              type="button"
              @click="emit('close')"
            >
              Cancel
            </Button>
            <Button
              type="submit"
              :disabled="saving || !name || !platform || !project"
            >
              <Loader2
                v-if="saving"
                class="mr-2 h-4 w-4 animate-spin"
              />
              {{ isEdit ? 'Save Changes' : 'Create Account' }}
            </Button>
          </div>
        </form>
      </div>
    </div>
  </Teleport>
</template>
