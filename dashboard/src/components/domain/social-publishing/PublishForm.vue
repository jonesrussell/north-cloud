<script setup lang="ts">
import { ref, computed } from 'vue'
import { Loader2, Send } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import type { SocialAccount, PublishRequest, TargetConfig } from '@/types/socialPublisher'

interface Props {
  accounts: SocialAccount[]
  accountsLoading: boolean
  publishing: boolean
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'publish', data: PublishRequest): void
}>()

const contentType = ref('social_update')
const title = ref('')
const body = ref('')
const summary = ref('')
const url = ref('')
const tags = ref('')
const project = ref('')
const source = ref('')
const selectedAccounts = ref<Set<string>>(new Set())
const scheduleMode = ref<'now' | 'later'>('now')
const scheduledAt = ref('')

const typeOptions = [
  { value: 'social_update', label: 'Social Update' },
  { value: 'blog_post', label: 'Blog Post' },
  { value: 'news_article', label: 'News Article' },
] as const

const canSubmit = computed(() => {
  return contentType.value && (title.value || body.value || summary.value)
})

function toggleAccount(accountName: string) {
  const next = new Set(selectedAccounts.value)
  if (next.has(accountName)) {
    next.delete(accountName)
  } else {
    next.add(accountName)
  }
  selectedAccounts.value = next
}

function handleSubmit() {
  const targets: TargetConfig[] = []
  for (const accountName of selectedAccounts.value) {
    const acct = props.accounts.find((a) => a.name === accountName)
    if (acct) {
      targets.push({ platform: acct.platform, account: acct.name })
    }
  }

  const data: PublishRequest = {
    type: contentType.value,
    title: title.value || undefined,
    body: body.value || undefined,
    summary: summary.value || undefined,
    url: url.value || undefined,
    tags: tags.value ? tags.value.split(',').map((t) => t.trim()).filter(Boolean) : undefined,
    project: project.value || undefined,
    source: source.value || undefined,
    targets: targets.length > 0 ? targets : undefined,
    scheduled_at: scheduleMode.value === 'later' && scheduledAt.value
      ? new Date(scheduledAt.value).toISOString()
      : undefined,
  }

  emit('publish', data)
}
</script>

<template>
  <form
    class="space-y-6"
    @submit.prevent="handleSubmit"
  >
    <div class="grid gap-4 sm:grid-cols-2">
      <div>
        <label class="mb-1 block text-sm font-medium">Type</label>
        <select
          v-model="contentType"
          class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
        >
          <option
            v-for="opt in typeOptions"
            :key="opt.value"
            :value="opt.value"
          >
            {{ opt.label }}
          </option>
        </select>
      </div>

      <div>
        <label class="mb-1 block text-sm font-medium">Project</label>
        <Input
          v-model="project"
          placeholder="e.g. personal"
        />
      </div>
    </div>

    <div>
      <label class="mb-1 block text-sm font-medium">Title</label>
      <Input
        v-model="title"
        placeholder="Content title"
      />
    </div>

    <div>
      <label class="mb-1 block text-sm font-medium">Body</label>
      <textarea
        v-model="body"
        rows="5"
        placeholder="Write your content..."
        class="flex w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
      />
    </div>

    <div>
      <label class="mb-1 block text-sm font-medium">Summary</label>
      <Input
        v-model="summary"
        placeholder="Brief summary"
      />
    </div>

    <div class="grid gap-4 sm:grid-cols-2">
      <div>
        <label class="mb-1 block text-sm font-medium">URL</label>
        <Input
          v-model="url"
          placeholder="https://..."
        />
      </div>
      <div>
        <label class="mb-1 block text-sm font-medium">Source</label>
        <Input
          v-model="source"
          placeholder="Content source"
        />
      </div>
    </div>

    <div>
      <label class="mb-1 block text-sm font-medium">Tags (comma-separated)</label>
      <Input
        v-model="tags"
        placeholder="tag1, tag2, tag3"
      />
    </div>

    <!-- Target Accounts -->
    <div>
      <label class="mb-2 block text-sm font-medium">Target Accounts</label>
      <div
        v-if="accountsLoading"
        class="text-sm text-muted-foreground"
      >
        <Loader2 class="mr-2 inline h-4 w-4 animate-spin" />
        Loading accounts...
      </div>
      <div
        v-else-if="accounts.length === 0"
        class="text-sm text-muted-foreground"
      >
        No accounts configured. <a
          href="/dashboard/distribution/social-accounts"
          class="text-primary hover:underline"
        >Add one first.</a>
      </div>
      <div
        v-else
        class="flex flex-wrap gap-2"
      >
        <button
          v-for="acct in accounts.filter(a => a.enabled)"
          :key="acct.id"
          type="button"
          :class="[
            'inline-flex items-center gap-2 rounded-lg border px-3 py-2 text-sm transition-colors',
            selectedAccounts.has(acct.name)
              ? 'border-primary bg-primary/10 text-primary'
              : 'border-muted hover:border-primary/50',
          ]"
          @click="toggleAccount(acct.name)"
        >
          <input
            type="checkbox"
            :checked="selectedAccounts.has(acct.name)"
            class="h-4 w-4 rounded border-gray-300"
            @click.stop
            @change="toggleAccount(acct.name)"
          >
          {{ acct.name }}
          <Badge
            variant="outline"
            class="text-xs"
          >
            {{ acct.platform }}
          </Badge>
        </button>
      </div>
    </div>

    <!-- Schedule -->
    <div>
      <label class="mb-2 block text-sm font-medium">Schedule</label>
      <div class="flex items-center gap-4">
        <button
          type="button"
          :class="[
            'inline-flex items-center rounded-full px-3 py-1.5 text-sm font-medium transition-colors',
            scheduleMode === 'now'
              ? 'bg-primary text-primary-foreground'
              : 'bg-muted text-muted-foreground hover:bg-muted/80',
          ]"
          @click="scheduleMode = 'now'"
        >
          Publish Now
        </button>
        <button
          type="button"
          :class="[
            'inline-flex items-center rounded-full px-3 py-1.5 text-sm font-medium transition-colors',
            scheduleMode === 'later'
              ? 'bg-primary text-primary-foreground'
              : 'bg-muted text-muted-foreground hover:bg-muted/80',
          ]"
          @click="scheduleMode = 'later'"
        >
          Schedule for Later
        </button>
      </div>
      <div
        v-if="scheduleMode === 'later'"
        class="mt-3"
      >
        <Input
          v-model="scheduledAt"
          type="datetime-local"
          required
        />
      </div>
    </div>

    <!-- Submit -->
    <div class="flex justify-end pt-2">
      <Button
        type="submit"
        :disabled="publishing || !canSubmit"
        size="lg"
      >
        <Loader2
          v-if="publishing"
          class="mr-2 h-4 w-4 animate-spin"
        />
        <Send
          v-else
          class="mr-2 h-4 w-4"
        />
        {{ scheduleMode === 'later' ? 'Schedule' : 'Publish' }}
      </Button>
    </div>
  </form>
</template>
