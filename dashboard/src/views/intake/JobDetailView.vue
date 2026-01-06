<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Pause, Play, XCircle, Loader2 } from 'lucide-vue-next'
import { crawlerApi } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'

const route = useRoute()
const router = useRouter()

const jobId = computed(() => String(route.params.id))

const loading = ref(true)
const error = ref<string | null>(null)
const job = ref<Record<string, unknown> | null>(null)
const stats = ref<Record<string, unknown> | null>(null)
const executions = ref<Array<Record<string, unknown>>>([])
const loadingExecutions = ref(false)

const loadJob = async () => {
  try {
    loading.value = true
    error.value = null
    const response = await crawlerApi.jobs.get(jobId.value)
    job.value = response.data
  } catch (err) {
    error.value = 'Unable to load job details.'
  } finally {
    loading.value = false
  }
}

const loadStats = async () => {
  try {
    const response = await crawlerApi.jobs.stats(jobId.value)
    stats.value = response.data
  } catch (err) {
    console.error('Error loading stats:', err)
  }
}

const loadExecutions = async () => {
  try {
    loadingExecutions.value = true
    const response = await crawlerApi.jobs.executions(jobId.value, { limit: 20 })
    executions.value = response.data?.executions || []
  } catch (err) {
    console.error('Error loading executions:', err)
  } finally {
    loadingExecutions.value = false
  }
}

const pauseJob = async () => {
  await crawlerApi.jobs.pause(jobId.value)
  await loadJob()
}

const resumeJob = async () => {
  await crawlerApi.jobs.resume(jobId.value)
  await loadJob()
}

const cancelJob = async () => {
  await crawlerApi.jobs.cancel(jobId.value)
  await loadJob()
}

const getStatusVariant = (status: string) => {
  switch (status) {
    case 'completed': return 'success'
    case 'running': return 'default'
    case 'failed': return 'destructive'
    default: return 'secondary'
  }
}

const formatDate = (date: string) => date ? new Date(date).toLocaleString() : 'N/A'
const formatDuration = (ms: number) => {
  if (!ms) return 'N/A'
  if (ms < 1000) return `${ms}ms`
  return `${(ms / 1000).toFixed(1)}s`
}

const canPause = computed(() => job.value && ['pending', 'scheduled'].includes(job.value.status as string) && !job.value.is_paused)
const canResume = computed(() => job.value?.is_paused)
const canCancel = computed(() => job.value && ['pending', 'scheduled', 'running'].includes(job.value.status as string))

onMounted(() => {
  loadJob()
  loadStats()
  loadExecutions()
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex items-center justify-between">
      <div class="flex items-center gap-4">
        <Button
          variant="ghost"
          size="icon"
          @click="router.push('/intake/jobs')"
        >
          <ArrowLeft class="h-5 w-5" />
        </Button>
        <div>
          <h1 class="text-3xl font-bold tracking-tight">
            {{ (job as Record<string, unknown>)?.source_name || 'Job Details' }}
          </h1>
          <p class="text-muted-foreground">
            Job ID: {{ jobId }}
          </p>
        </div>
      </div>
      <div class="flex gap-2">
        <Button
          v-if="canPause"
          variant="outline"
          @click="pauseJob"
        >
          <Pause class="mr-2 h-4 w-4" /> Pause
        </Button>
        <Button
          v-if="canResume"
          variant="outline"
          @click="resumeJob"
        >
          <Play class="mr-2 h-4 w-4" /> Resume
        </Button>
        <Button
          v-if="canCancel"
          variant="destructive"
          @click="cancelJob"
        >
          <XCircle class="mr-2 h-4 w-4" /> Cancel
        </Button>
      </div>
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

    <!-- Job Details -->
    <template v-else-if="job">
      <!-- Info Card -->
      <Card>
        <CardHeader>
          <CardTitle>Job Information</CardTitle>
        </CardHeader>
        <CardContent>
          <dl class="grid grid-cols-2 gap-4">
            <div>
              <dt class="text-sm text-muted-foreground">
                Status
              </dt>
              <dd class="mt-1">
                <Badge :variant="getStatusVariant((job as Record<string, unknown>).status as string)">
                  {{ (job as Record<string, unknown>).status }}
                </Badge>
              </dd>
            </div>
            <div>
              <dt class="text-sm text-muted-foreground">
                Source
              </dt>
              <dd class="mt-1">
                {{ (job as Record<string, unknown>).source_name || 'N/A' }}
              </dd>
            </div>
            <div class="col-span-2">
              <dt class="text-sm text-muted-foreground">
                URL
              </dt>
              <dd class="mt-1">
                <a
                  :href="(job as Record<string, unknown>).url as string"
                  target="_blank"
                  class="text-primary hover:underline break-all"
                >
                  {{ (job as Record<string, unknown>).url }}
                </a>
              </dd>
            </div>
            <div>
              <dt class="text-sm text-muted-foreground">
                Created
              </dt>
              <dd class="mt-1">
                {{ formatDate((job as Record<string, unknown>).created_at as string) }}
              </dd>
            </div>
            <div v-if="(job as Record<string, unknown>).schedule_enabled">
              <dt class="text-sm text-muted-foreground">
                Schedule
              </dt>
              <dd class="mt-1">
                Every {{ (job as Record<string, unknown>).interval_minutes }} {{ (job as Record<string, unknown>).interval_type }}
              </dd>
            </div>
          </dl>
        </CardContent>
      </Card>

      <!-- Statistics -->
      <Card v-if="stats">
        <CardHeader>
          <CardTitle>Statistics</CardTitle>
        </CardHeader>
        <CardContent>
          <div class="grid grid-cols-3 gap-4">
            <div>
              <p class="text-sm text-muted-foreground">
                Total Executions
              </p>
              <p class="text-2xl font-bold">
                {{ (stats as Record<string, unknown>).total_executions || 0 }}
              </p>
            </div>
            <div>
              <p class="text-sm text-muted-foreground">
                Successful
              </p>
              <p class="text-2xl font-bold text-green-600">
                {{ (stats as Record<string, unknown>).successful_runs || 0 }}
              </p>
            </div>
            <div>
              <p class="text-sm text-muted-foreground">
                Failed
              </p>
              <p class="text-2xl font-bold text-red-600">
                {{ (stats as Record<string, unknown>).failed_runs || 0 }}
              </p>
            </div>
          </div>
        </CardContent>
      </Card>

      <!-- Executions -->
      <Card>
        <CardHeader>
          <CardTitle>Execution History</CardTitle>
        </CardHeader>
        <CardContent class="p-0">
          <div
            v-if="loadingExecutions"
            class="flex justify-center py-8"
          >
            <Loader2 class="h-6 w-6 animate-spin" />
          </div>
          <div
            v-else-if="executions.length === 0"
            class="py-8 text-center text-muted-foreground"
          >
            No executions yet
          </div>
          <table
            v-else
            class="w-full"
          >
            <thead class="border-b bg-muted/50">
              <tr>
                <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                  #
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                  Status
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                  Started
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                  Duration
                </th>
              </tr>
            </thead>
            <tbody class="divide-y">
              <tr
                v-for="exec in executions"
                :key="(exec as Record<string, unknown>).id as string"
              >
                <td class="px-6 py-4 text-sm">
                  #{{ (exec as Record<string, unknown>).execution_number }}
                </td>
                <td class="px-6 py-4">
                  <Badge :variant="getStatusVariant((exec as Record<string, unknown>).status as string)">
                    {{ (exec as Record<string, unknown>).status }}
                  </Badge>
                </td>
                <td class="px-6 py-4 text-sm text-muted-foreground">
                  {{ formatDate((exec as Record<string, unknown>).started_at as string) }}
                </td>
                <td class="px-6 py-4 text-sm text-muted-foreground">
                  {{ formatDuration((exec as Record<string, unknown>).duration_ms as number) }}
                </td>
              </tr>
            </tbody>
          </table>
        </CardContent>
      </Card>
    </template>
  </div>
</template>
