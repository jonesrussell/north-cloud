<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { Plus, Briefcase, Loader2 } from 'lucide-vue-next'
import { crawlerApi, sourcesApi } from '@/api/client'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Input } from '@/components/ui/input'

interface Job {
  id: string
  source_name: string
  source_id: string
  url: string
  status: string
  created_at: string
  next_run_at?: string
  schedule_enabled: boolean
  interval_minutes?: number
  interval_type?: string
}

interface Source {
  id: string
  name: string
  url: string
}

const router = useRouter()

const loading = ref(true)
const error = ref<string | null>(null)
const jobs = ref<Job[]>([])
const showCreateModal = ref(false)
const creating = ref(false)
const createError = ref<string | null>(null)
const createSuccess = ref(false)
const deleting = ref(false)
const showDeleteModal = ref(false)
const jobToDelete = ref<Job | null>(null)

// Sources data
const sources = ref<Source[]>([])
const loadingSources = ref(false)
const selectedSource = ref<Source | null>(null)

// New job form
const newJob = ref({
  source_id: '',
  url: '',
  interval_minutes: 30,
  interval_type: 'minutes',
  schedule_enabled: false,
})

const loadSources = async () => {
  try {
    loadingSources.value = true
    const response = await sourcesApi.list()
    sources.value = response.data?.sources || response.data || []
  } catch (err) {
    console.error('Error loading sources:', err)
  } finally {
    loadingSources.value = false
  }
}

const loadJobs = async () => {
  try {
    loading.value = true
    error.value = null
    const response = await crawlerApi.jobs.list()
    jobs.value = response.data?.jobs || response.data || []
  } catch (err) {
    error.value = 'Unable to load jobs. Backend API may not be available.'
    console.error('Error loading jobs:', err)
  } finally {
    loading.value = false
  }
}

const onSourceChange = (e: Event) => {
  const target = e.target as HTMLSelectElement
  newJob.value.source_id = target.value
  selectedSource.value = sources.value.find((s) => s.id === target.value) || null
  if (selectedSource.value) {
    newJob.value.url = selectedSource.value.url
  }
}

const createJob = async () => {
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

    await crawlerApi.jobs.create(jobData)
    createSuccess.value = true
    await loadJobs()
    
    setTimeout(() => {
      showCreateModal.value = false
      newJob.value = { source_id: '', url: '', interval_minutes: 30, interval_type: 'minutes', schedule_enabled: false }
      selectedSource.value = null
      createSuccess.value = false
    }, 1000)
  } catch (err: unknown) {
    const error = err as { response?: { data?: { error?: string } } }
    createError.value = error.response?.data?.error || 'Failed to create job'
  } finally {
    creating.value = false
  }
}

const confirmDelete = (job: Job) => {
  jobToDelete.value = job
  showDeleteModal.value = true
}

const deleteJob = async () => {
  if (!jobToDelete.value) return
  try {
    deleting.value = true
    await crawlerApi.jobs.delete(jobToDelete.value.id)
    jobs.value = jobs.value.filter((j) => j.id !== jobToDelete.value?.id)
    showDeleteModal.value = false
  } catch (err) {
    console.error('Error deleting job:', err)
  } finally {
    deleting.value = false
    jobToDelete.value = null
  }
}

const getStatusVariant = (status: string) => {
  switch (status) {
    case 'completed': return 'success'
    case 'running': return 'default'
    case 'failed': return 'destructive'
    case 'cancelled': return 'secondary'
    case 'paused': return 'warning'
    default: return 'pending'
  }
}

const truncateId = (id: string) => id.length <= 12 ? id : `${id.substring(0, 8)}...`
const formatDate = (date: string) => date ? new Date(date).toLocaleString() : 'N/A'
const formatNextRun = (job: Job) => {
  if (!job.schedule_enabled) return job.status === 'pending' ? 'Pending' : 'N/A'
  return job.next_run_at ? new Date(job.next_run_at).toLocaleString() : 'N/A'
}

const navigateToJob = (jobId: string) => router.push(`/intake/jobs/${jobId}`)

watch(showCreateModal, (val) => {
  if (val && sources.value.length === 0) loadSources()
})

onMounted(() => {
  loadJobs()
  loadSources()
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
      <Button @click="showCreateModal = true">
        <Plus class="mr-2 h-4 w-4" />
        Create Job
      </Button>
    </div>

    <!-- Loading -->
    <div
      v-if="loading"
      class="flex items-center justify-center py-12"
    >
      <Loader2 class="h-8 w-8 animate-spin text-muted-foreground" />
    </div>

    <!-- Error -->
    <Card
      v-else-if="error"
      class="border-destructive"
    >
      <CardContent class="pt-6">
        <p class="text-destructive">
          {{ error }}
        </p>
      </CardContent>
    </Card>

    <!-- Empty state -->
    <Card v-else-if="jobs.length === 0">
      <CardContent class="flex flex-col items-center justify-center py-12">
        <Briefcase class="h-12 w-12 text-muted-foreground mb-4" />
        <h3 class="text-lg font-medium mb-2">
          No crawl jobs
        </h3>
        <p class="text-muted-foreground mb-4">
          Get started by creating your first crawl job.
        </p>
        <Button @click="showCreateModal = true">
          <Plus class="mr-2 h-4 w-4" />
          Create Job
        </Button>
      </CardContent>
    </Card>

    <!-- Jobs table -->
    <Card v-else>
      <CardContent class="p-0">
        <table class="w-full">
          <thead class="border-b bg-muted/50">
            <tr>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Job ID
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Source
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Status
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Created
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                Next Run
              </th>
              <th class="px-6 py-3 text-right text-xs font-medium text-muted-foreground uppercase">
                Actions
              </th>
            </tr>
          </thead>
          <tbody class="divide-y">
            <tr
              v-for="job in jobs"
              :key="job.id"
              class="hover:bg-muted/50 cursor-pointer"
              @click="navigateToJob(job.id)"
            >
              <td class="px-6 py-4 text-sm font-medium">
                <button
                  class="text-primary hover:underline"
                  @click.stop="navigateToJob(job.id)"
                >
                  {{ truncateId(job.id) }}
                </button>
              </td>
              <td class="px-6 py-4 text-sm text-muted-foreground">
                {{ job.source_name || 'N/A' }}
              </td>
              <td class="px-6 py-4">
                <Badge :variant="getStatusVariant(job.status)">
                  {{ job.status }}
                </Badge>
              </td>
              <td class="px-6 py-4 text-sm text-muted-foreground">
                {{ formatDate(job.created_at) }}
              </td>
              <td class="px-6 py-4 text-sm text-muted-foreground">
                {{ formatNextRun(job) }}
              </td>
              <td class="px-6 py-4 text-right">
                <Button
                  variant="ghost"
                  size="sm"
                  class="text-destructive"
                  @click.stop="confirmDelete(job)"
                >
                  Delete
                </Button>
              </td>
            </tr>
          </tbody>
        </table>
      </CardContent>
    </Card>

    <!-- Create Modal -->
    <div
      v-if="showCreateModal"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
    >
      <Card class="w-full max-w-md mx-4">
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
              <label class="block text-sm font-medium mb-2">Source</label>
              <select
                :value="newJob.source_id"
                class="w-full px-3 py-2 border rounded-md bg-background"
                @change="onSourceChange"
              >
                <option value="">
                  Select a source...
                </option>
                <option
                  v-for="s in sources"
                  :key="s.id"
                  :value="s.id"
                >
                  {{ s.name }}
                </option>
              </select>
            </div>

            <div>
              <label class="block text-sm font-medium mb-2">URL</label>
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
                class="flex-1 px-3 py-2 border rounded-md bg-background"
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
              class="p-3 text-sm text-destructive bg-destructive/10 rounded-md"
            >
              {{ createError }}
            </div>

            <div
              v-if="createSuccess"
              class="p-3 text-sm text-green-600 bg-green-50 rounded-md"
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
      <Card class="w-full max-w-md mx-4">
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
              @click="deleteJob"
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
