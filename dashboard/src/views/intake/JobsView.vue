<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { Plus, Briefcase, Loader2, RefreshCw } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { JobsFilterBar, JobsTable, JobStatsCard } from '@/components/domain/jobs'
import { useJobsStore, useSourcesStore } from '@/stores'
import type { Job } from '@/types/crawler'

const router = useRouter()
const route = useRoute()
const jobsStore = useJobsStore()
const sourcesStore = useSourcesStore()

// Modal state
const showCreateModal = ref(false)
const creating = ref(false)
const createError = ref<string | null>(null)
const createSuccess = ref(false)
const showDeleteModal = ref(false)
const deleting = ref(false)
const jobToDelete = ref<Job | null>(null)

// Selected source for create modal
const selectedSource = computed(() =>
  sourcesStore.getSourceById(newJob.value.source_id)
)

// New job form
const newJob = ref({
  source_id: '',
  url: '',
  interval_minutes: 30,
  interval_type: 'minutes' as const,
  schedule_enabled: false,
})

// Polling interval
const POLL_INTERVAL = 30000

function onSourceChange(e: Event) {
  const target = e.target as HTMLSelectElement
  newJob.value.source_id = target.value
  const source = sourcesStore.getSourceById(target.value)
  if (source) {
    newJob.value.url = source.url
  }
}

async function createJob() {
  createError.value = null
  createSuccess.value = false

  if (!newJob.value.source_id) {
    createError.value = 'Please select a source'
    return
  }

  try {
    creating.value = true
    const jobData = {
      source_id: newJob.value.source_id,
      source_name: selectedSource.value?.name || '',
      url: newJob.value.url,
      schedule_enabled: newJob.value.schedule_enabled,
      ...(newJob.value.schedule_enabled && {
        interval_minutes: newJob.value.interval_minutes,
        interval_type: newJob.value.interval_type,
      }),
    }

    await jobsStore.createJob(jobData)
    createSuccess.value = true

    setTimeout(() => {
      showCreateModal.value = false
      resetCreateForm()
    }, 1000)
  } catch (err: unknown) {
    const error = err as { response?: { data?: { error?: string } } }
    createError.value = error.response?.data?.error || 'Failed to create job'
  } finally {
    creating.value = false
  }
}

function resetCreateForm() {
  newJob.value = {
    source_id: '',
    url: '',
    interval_minutes: 30,
    interval_type: 'minutes',
    schedule_enabled: false,
  }
  createSuccess.value = false
  createError.value = null
}

function confirmDelete(job: Job) {
  jobToDelete.value = job
  showDeleteModal.value = true
}

async function handleDelete() {
  if (!jobToDelete.value) return
  try {
    deleting.value = true
    await jobsStore.deleteJob(jobToDelete.value.id)
    showDeleteModal.value = false
  } catch (err) {
    console.error('Failed to delete job:', err)
  } finally {
    deleting.value = false
    jobToDelete.value = null
  }
}

// Table event handlers
function handleView(job: Job) {
  router.push({ name: 'intake-job-detail', params: { id: job.id } })
}

async function handlePause(job: Job) {
  await jobsStore.pauseJob(job.id)
}

async function handleResume(job: Job) {
  await jobsStore.resumeJob(job.id)
}

async function handleCancel(job: Job) {
  await jobsStore.cancelJob(job.id)
}

async function handleRetry(job: Job) {
  await jobsStore.retryJob(job.id)
}

watch(showCreateModal, (val) => {
  if (val && sourcesStore.items.length === 0) {
    sourcesStore.fetchSources()
  }
})

onMounted(async () => {
  await jobsStore.fetchJobs()
  await sourcesStore.fetchSources()
  jobsStore.startPolling(POLL_INTERVAL)

  // Auto-open create modal if ?create=true
  if (route.query.create === 'true') {
    showCreateModal.value = true
    router.replace({ query: {} })
  }
})

onUnmounted(() => {
  jobsStore.stopPolling()
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-3xl font-bold tracking-tight">
          Crawl Jobs
        </h1>
        <p class="text-muted-foreground">
          Manage and monitor content crawling jobs
        </p>
      </div>
      <div class="flex items-center gap-2">
        <Button
          variant="outline"
          size="sm"
          @click="jobsStore.fetchJobs()"
        >
          <RefreshCw :class="['mr-2 h-4 w-4', jobsStore.loading && 'animate-spin']" />
          Refresh
        </Button>
        <Button @click="showCreateModal = true">
          <Plus class="mr-2 h-4 w-4" />
          Create Job
        </Button>
      </div>
    </div>

    <!-- Stats Cards -->
    <JobStatsCard />

    <!-- Error -->
    <Card
      v-if="jobsStore.error"
      class="border-destructive"
    >
      <CardContent class="pt-6">
        <p class="text-destructive">
          {{ jobsStore.error }}
        </p>
      </CardContent>
    </Card>

    <!-- Empty state (only show when no jobs at all) -->
    <Card v-else-if="!jobsStore.loading && jobsStore.items.length === 0">
      <CardContent class="flex flex-col items-center justify-center py-12">
        <Briefcase class="mb-4 h-12 w-12 text-muted-foreground" />
        <h3 class="mb-2 text-lg font-medium">
          No crawl jobs
        </h3>
        <p class="mb-4 text-muted-foreground">
          Get started by creating your first crawl job.
        </p>
        <Button @click="showCreateModal = true">
          <Plus class="mr-2 h-4 w-4" />
          Create Job
        </Button>
      </CardContent>
    </Card>

    <!-- Filter Bar + Table -->
    <template v-else>
      <Card>
        <CardHeader class="pb-4">
          <CardTitle class="text-base">
            Filter Jobs
          </CardTitle>
        </CardHeader>
        <CardContent>
          <JobsFilterBar
            show-source-filter
            :sources="sourcesStore.sourceOptions"
          />
        </CardContent>
      </Card>

      <Card>
        <CardContent class="p-0">
          <JobsTable
            @view="handleView"
            @pause="handlePause"
            @resume="handleResume"
            @cancel="handleCancel"
            @retry="handleRetry"
            @delete="confirmDelete"
          />
        </CardContent>
      </Card>
    </template>

    <!-- Create Modal -->
    <div
      v-if="showCreateModal"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
    >
      <Card class="mx-4 w-full max-w-md">
        <CardHeader>
          <CardTitle>Create Crawl Job</CardTitle>
          <CardDescription>Create a new job to crawl content from a source</CardDescription>
        </CardHeader>
        <CardContent>
          <form
            class="space-y-4"
            @submit.prevent="createJob"
          >
            <div>
              <label class="mb-2 block text-sm font-medium">Source</label>
              <select
                :value="newJob.source_id"
                class="w-full rounded-md border bg-background px-3 py-2"
                @change="onSourceChange"
              >
                <option value="">
                  Select a source...
                </option>
                <option
                  v-for="s in sourcesStore.items"
                  :key="s.id"
                  :value="s.id"
                >
                  {{ s.name }}
                </option>
              </select>
            </div>

            <div>
              <label class="mb-2 block text-sm font-medium">URL</label>
              <Input
                :model-value="newJob.url"
                disabled
                placeholder="Select a source"
              />
            </div>

            <div class="flex items-center gap-2">
              <input
                id="schedule_enabled"
                v-model="newJob.schedule_enabled"
                type="checkbox"
                class="h-4 w-4"
              >
              <label
                for="schedule_enabled"
                class="text-sm"
              >Enable scheduled crawling</label>
            </div>

            <div
              v-if="newJob.schedule_enabled"
              class="flex gap-3"
            >
              <Input
                v-model.number="newJob.interval_minutes"
                type="number"
                min="1"
                class="flex-1"
              />
              <select
                v-model="newJob.interval_type"
                class="flex-1 rounded-md border bg-background px-3 py-2"
              >
                <option value="minutes">
                  Minutes
                </option>
                <option value="hours">
                  Hours
                </option>
                <option value="days">
                  Days
                </option>
              </select>
            </div>

            <div
              v-if="createError"
              class="rounded-md bg-destructive/10 p-3 text-sm text-destructive"
            >
              {{ createError }}
            </div>

            <div
              v-if="createSuccess"
              class="rounded-md bg-green-50 p-3 text-sm text-green-600"
            >
              Job created successfully!
            </div>

            <div class="flex justify-end gap-3 pt-4">
              <Button
                type="button"
                variant="outline"
                @click="showCreateModal = false"
              >
                Cancel
              </Button>
              <Button
                type="submit"
                :disabled="creating"
              >
                <Loader2
                  v-if="creating"
                  class="mr-2 h-4 w-4 animate-spin"
                />
                {{ creating ? 'Creating...' : 'Create Job' }}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>
    </div>

    <!-- Delete Modal -->
    <div
      v-if="showDeleteModal"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
    >
      <Card class="mx-4 w-full max-w-md">
        <CardHeader>
          <CardTitle>Delete Job</CardTitle>
          <CardDescription>Are you sure you want to delete this job? This action cannot be undone.</CardDescription>
        </CardHeader>
        <CardContent>
          <div class="flex justify-end gap-3">
            <Button
              variant="outline"
              @click="showDeleteModal = false"
            >
              Cancel
            </Button>
            <Button
              variant="destructive"
              :disabled="deleting"
              @click="handleDelete"
            >
              <Loader2
                v-if="deleting"
                class="mr-2 h-4 w-4 animate-spin"
              />
              Delete
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
