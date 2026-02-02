<script setup lang="ts">
/**
 * Jobs View (Refactored)
 *
 * Uses the new feature module architecture:
 * - TanStack Query for server state (jobs list, loading, error)
 * - useJobsQueryStore for filters and pagination
 * - useJobsUIStore for modals and selections
 * - useJobs composable combines everything
 * - useSources composable for source dropdown
 */
import { ref, computed, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { Plus, Briefcase, Loader2, RefreshCw } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { JobsFilterBar, JobsTable, JobStatsCard } from '@/components/domain/jobs'
import { LiveUpdateIndicator } from '@/components/domain/realtime'

// Import from new feature modules
import { useJobs } from '@/features/intake'
import { useSources } from '@/features/scheduling'
import type { Job, CreateJobRequest } from '@/types/crawler'

const router = useRouter()
const route = useRoute()

// Use the new combined composables
const jobs = useJobs()
const sources = useSources()

// Computed for update mode display
const updateMode = computed(() => jobs.isFetching.value ? 'Updating...' : 'Polling')

// Selected source for create modal
const selectedSource = computed(() =>
  sources.getSourceById(newJob.value.source_id)
)

// New job form state (local to this component)
const newJob = ref({
  source_id: '',
  url: '',
  interval_minutes: 30,
  interval_type: 'minutes' as const,
  schedule_enabled: false,
})

// Local state for create form
const createError = ref<string | null>(null)
const createSuccess = ref(false)

// Job to delete (for confirmation modal)
const jobToDelete = ref<Job | null>(null)

function onSourceChange(e: Event) {
  const target = e.target as HTMLSelectElement
  newJob.value.source_id = target.value
  const source = sources.getSourceById(target.value)
  if (source) {
    newJob.value.url = source.url
  }
}

async function handleCreateJob() {
  createError.value = null
  createSuccess.value = false

  if (!newJob.value.source_id) {
    createError.value = 'Please select a source'
    return
  }

  try {
    const jobData: CreateJobRequest = {
      source_id: newJob.value.source_id,
      source_name: selectedSource.value?.name || '',
      url: newJob.value.url,
      schedule_enabled: newJob.value.schedule_enabled,
      ...(newJob.value.schedule_enabled && {
        interval_minutes: newJob.value.interval_minutes,
        interval_type: newJob.value.interval_type,
      }),
    }

    await jobs.createJob(jobData)
    createSuccess.value = true

    setTimeout(() => {
      jobs.ui.closeModal('create')
      resetCreateForm()
    }, 1000)
  } catch (err: unknown) {
    const error = err as { response?: { data?: { error?: string } }; message?: string }
    createError.value = error.response?.data?.error || error.message || 'Failed to create job'
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
  jobs.ui.selectJob(job.id, job)
  jobs.ui.openModal('delete')
}

async function handleDelete() {
  if (!jobToDelete.value) return
  try {
    await jobs.deleteJob(jobToDelete.value.id)
    jobToDelete.value = null
  } catch (err) {
    console.error('Failed to delete job:', err)
  }
}

// Table event handlers
function handleView(job: Job) {
  router.push({ name: 'intake-job-detail', params: { id: job.id } })
}

async function handlePause(job: Job) {
  await jobs.pauseJob(job.id)
}

async function handleResume(job: Job) {
  await jobs.resumeJob(job.id)
}

async function handleCancel(job: Job) {
  await jobs.cancelJob(job.id)
}

async function handleRetry(job: Job) {
  await jobs.retryJob(job.id)
}

async function handleRunNow(job: Job) {
  await jobs.forceRunJob(job.id)
}

// Auto-open create modal if ?create=true in URL
watch(() => route.query.create, (create) => {
  if (create === 'true') {
    jobs.ui.openModal('create')
    router.replace({ query: {} })
  }
}, { immediate: true })

// Sources are fetched automatically by TanStack Query via useSources()
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
      <div class="flex items-center gap-3">
        <!-- Update mode indicator -->
        <div class="flex items-center gap-2 text-sm text-muted-foreground">
          <LiveUpdateIndicator :event-types="['job:status', 'job:completed']" />
          <span>{{ updateMode }}</span>
        </div>

        <Button
          variant="outline"
          size="sm"
          :disabled="jobs.isFetching.value"
          @click="jobs.refetch()"
        >
          <RefreshCw :class="['mr-2 h-4 w-4', jobs.isFetching.value && 'animate-spin']" />
          Refresh
        </Button>
        <Button @click="jobs.ui.openModal('create')">
          <Plus class="mr-2 h-4 w-4" />
          Create Job
        </Button>
      </div>
    </div>

    <!-- Stats Cards -->
    <JobStatsCard />

    <!-- Error State -->
    <Card
      v-if="jobs.error.value"
      class="border-destructive"
    >
      <CardContent class="pt-6">
        <p class="text-destructive">
          {{ jobs.error.value?.message || 'Failed to load jobs. Please check if the crawler service is running.' }}
        </p>
      </CardContent>
    </Card>

    <!-- Loading State -->
    <Card v-else-if="jobs.isLoading.value">
      <CardContent class="flex flex-col items-center justify-center py-12">
        <Loader2 class="mb-4 h-8 w-8 animate-spin text-muted-foreground" />
        <p class="text-muted-foreground">
          Loading jobs...
        </p>
      </CardContent>
    </Card>

    <!-- Empty state (only show when no jobs at all) -->
    <Card v-else-if="jobs.jobs.value.length === 0">
      <CardContent class="flex flex-col items-center justify-center py-12">
        <Briefcase class="mb-4 h-12 w-12 text-muted-foreground" />
        <h3 class="mb-2 text-lg font-medium">
          No crawl jobs
        </h3>
        <p class="mb-4 text-muted-foreground">
          Get started by creating your first crawl job.
        </p>
        <Button @click="jobs.ui.openModal('create')">
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
            :sources="sources.sourceOptions.value"
          />
        </CardContent>
      </Card>

      <Card>
        <CardContent class="p-0">
          <JobsTable
            @view="handleView"
            @pause="handlePause"
            @resume="handleResume"
            @run-now="handleRunNow"
            @cancel="handleCancel"
            @retry="handleRetry"
            @delete="confirmDelete"
          />
        </CardContent>
      </Card>
    </template>

    <!-- Create Modal -->
    <div
      v-if="jobs.ui.modals.create"
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
            @submit.prevent="handleCreateJob"
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
                  v-for="s in sources.sources.value"
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
                @click="jobs.ui.closeModal('create')"
              >
                Cancel
              </Button>
              <Button
                type="submit"
                :disabled="jobs.isCreating.value"
              >
                <Loader2
                  v-if="jobs.isCreating.value"
                  class="mr-2 h-4 w-4 animate-spin"
                />
                {{ jobs.isCreating.value ? 'Creating...' : 'Create Job' }}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>
    </div>

    <!-- Delete Modal -->
    <div
      v-if="jobs.ui.modals.delete"
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
              @click="jobs.ui.closeModal('delete')"
            >
              Cancel
            </Button>
            <Button
              variant="destructive"
              :disabled="jobs.isDeleting.value"
              @click="handleDelete"
            >
              <Loader2
                v-if="jobs.isDeleting.value"
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
