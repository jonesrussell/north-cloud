<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { formatDate, formatRelativeTime } from '@/lib/utils'
import {
  Play,
  PlayCircle,
  Pause,
  XCircle,
  RotateCcw,
  ChevronLeft,
  ChevronRight,
  MoreHorizontal,
  ExternalLink,
  Clock,
  AlertTriangle,
  ArrowUp,
  ArrowDown,
  ArrowUpDown,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Skeleton } from '@/components/ui/skeleton'
import type { JobsComposable } from '@/features/intake'
import type { Job, JobStatus } from '@/types/crawler'

interface Props {
  jobs: JobsComposable
  showActions?: boolean
  onRowClick?: (job: Job) => void
}

const props = withDefaults(defineProps<Props>(), {
  showActions: true,
  onRowClick: undefined,
})

const emit = defineEmits<{
  (e: 'view', job: Job): void
  (e: 'pause', job: Job): void
  (e: 'resume', job: Job): void
  (e: 'runNow', job: Job): void
  (e: 'cancel', job: Job): void
  (e: 'retry', job: Job): void
  (e: 'delete', job: Job): void
}>()

const router = useRouter()

type BadgeVariant = 'default' | 'secondary' | 'destructive' | 'outline' | 'success' | 'warning' | 'pending'

const statusVariants: Record<JobStatus, BadgeVariant> = {
  running: 'default',
  scheduled: 'secondary',
  pending: 'pending',
  completed: 'success',
  failed: 'destructive',
  paused: 'warning',
  cancelled: 'outline',
}

// Define sortable columns
const sortableColumns = [
  { key: 'source_name', label: 'Source' },
  { key: 'status', label: 'Status' },
  { key: 'next_run_at', label: 'Next Run' },
  { key: 'last_run_at', label: 'Last Run' },
] as const

function getSortIcon(column: string) {
  if (props.jobs.sortBy.value !== column) return ArrowUpDown
  return props.jobs.sortOrder.value === 'asc' ? ArrowUp : ArrowDown
}

function handleSort(column: string) {
  props.jobs.toggleSort(column)
}

const pageNumbers = computed(() => {
  const current = props.jobs.page.value
  const total = props.jobs.totalPages.value
  const pages: (number | string)[] = []

  if (total <= 7) {
    for (let i = 1; i <= total; i++) pages.push(i)
  } else {
    pages.push(1)
    if (current > 3) pages.push('...')
    for (let i = Math.max(2, current - 1); i <= Math.min(total - 1, current + 1); i++) {
      pages.push(i)
    }
    if (current < total - 2) pages.push('...')
    pages.push(total)
  }

  return pages
})

function truncateId(id: string): string {
  return id.length > 8 ? `${id.slice(0, 8)}...` : id
}

function canPause(job: Job): boolean {
  return ['pending', 'scheduled'].includes(job.status)
}

function canResume(job: Job): boolean {
  return job.status === 'paused'
}

function canCancel(job: Job): boolean {
  return ['pending', 'scheduled', 'running'].includes(job.status)
}

function canRetry(job: Job): boolean {
  return ['failed', 'cancelled'].includes(job.status)
}

function canRunNow(job: Job): boolean {
  return ['scheduled', 'paused', 'pending'].includes(job.status)
}

function handleRowClick(job: Job) {
  if (props.onRowClick) {
    props.onRowClick(job)
  } else {
    router.push({ name: 'intake-job-detail', params: { id: job.id } })
  }
}

function goToPage(page: number | string) {
  if (typeof page === 'number') {
    props.jobs.setPage(page)
  }
}

function handlePageSizeChange(event: Event) {
  const target = event.target as HTMLSelectElement
  props.jobs.setPageSize(Number(target.value))
}
</script>

<template>
  <div class="space-y-4">
    <!-- Table -->
    <div class="rounded-md border">
      <table class="w-full">
        <thead>
          <tr class="border-b bg-muted/50">
            <th class="px-4 py-3 text-left text-sm font-medium text-muted-foreground">
              Job ID
            </th>
            <th
              v-for="col in sortableColumns"
              :key="col.key"
              class="px-4 py-3 text-left text-sm font-medium text-muted-foreground cursor-pointer hover:text-foreground transition-colors"
              @click="handleSort(col.key)"
            >
              <div class="flex items-center gap-1">
                {{ col.label }}
                <component
                  :is="getSortIcon(col.key)"
                  :class="[
                    'h-4 w-4',
                    jobs.sortBy.value === col.key ? 'text-foreground' : 'text-muted-foreground/50'
                  ]"
                />
              </div>
            </th>
            <th class="px-4 py-3 text-left text-sm font-medium text-muted-foreground">
              Schedule
            </th>
            <th
              v-if="showActions"
              class="px-4 py-3 text-right text-sm font-medium text-muted-foreground"
            >
              Actions
            </th>
          </tr>
        </thead>
        <tbody>
          <!-- Loading State -->
          <template v-if="jobs.isLoading.value">
            <tr
              v-for="i in 5"
              :key="i"
              class="border-b"
            >
              <td class="px-4 py-3">
                <Skeleton class="h-4 w-20" />
              </td>
              <td class="px-4 py-3">
                <Skeleton class="h-4 w-32" />
              </td>
              <td class="px-4 py-3">
                <Skeleton class="h-5 w-20" />
              </td>
              <td class="px-4 py-3">
                <Skeleton class="h-4 w-24" />
              </td>
              <td class="px-4 py-3">
                <Skeleton class="h-4 w-24" />
              </td>
              <td class="px-4 py-3">
                <Skeleton class="h-4 w-16" />
              </td>
              <td
                v-if="showActions"
                class="px-4 py-3"
              >
                <Skeleton class="ml-auto h-8 w-8" />
              </td>
            </tr>
          </template>

          <!-- Empty State -->
          <tr v-else-if="jobs.jobs.value.length === 0">
            <td
              :colspan="showActions ? 7 : 6"
              class="px-4 py-12 text-center"
            >
              <div class="flex flex-col items-center gap-2">
                <AlertTriangle class="h-8 w-8 text-muted-foreground" />
                <p class="text-sm text-muted-foreground">
                  {{ jobs.hasActiveFilters.value ? 'No jobs match your filters' : 'No jobs found' }}
                </p>
                <Button
                  v-if="jobs.hasActiveFilters.value"
                  variant="outline"
                  size="sm"
                  @click="jobs.clearAllFilters()"
                >
                  Clear filters
                </Button>
              </div>
            </td>
          </tr>

          <!-- Data Rows -->
          <tr
            v-for="job in jobs.jobs.value"
            v-else
            :key="job.id"
            class="border-b transition-colors hover:bg-muted/50 cursor-pointer"
            @click="handleRowClick(job)"
          >
            <td class="px-4 py-3">
              <code class="rounded bg-muted px-1.5 py-0.5 text-xs font-mono">
                {{ truncateId(job.id) }}
              </code>
            </td>
            <td class="px-4 py-3">
              <div class="flex flex-col">
                <span class="font-medium">{{ job.source_name }}</span>
                <a
                  :href="job.url"
                  target="_blank"
                  class="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
                  @click.stop
                >
                  <span class="max-w-48 truncate">{{ job.url }}</span>
                  <ExternalLink class="h-3 w-3" />
                </a>
              </div>
            </td>
            <td class="px-4 py-3">
              <Badge :variant="statusVariants[job.status]">
                {{ job.status }}
              </Badge>
            </td>
            <td class="px-4 py-3 text-sm">
              <span
                v-if="job.next_run_at"
                :class="{ 'text-yellow-600': job.status === 'scheduled' }"
              >
                {{ formatRelativeTime(job.next_run_at) }}
              </span>
              <span
                v-else
                class="text-muted-foreground"
              >—</span>
            </td>
            <td class="px-4 py-3 text-sm text-muted-foreground">
              {{ formatDate(job.started_at) }}
            </td>
            <td class="px-4 py-3">
              <div
                v-if="job.schedule_enabled"
                class="flex items-center gap-1.5 text-sm"
              >
                <Clock class="h-3.5 w-3.5 text-muted-foreground" />
                <span>{{ job.interval_minutes }} {{ job.interval_type }}</span>
              </div>
              <span
                v-else
                class="text-sm text-muted-foreground"
              >One-time</span>
            </td>
            <td
              v-if="showActions"
              class="px-4 py-3 text-right"
              @click.stop
            >
              <DropdownMenu>
                <DropdownMenuTrigger as-child>
                  <Button
                    variant="ghost"
                    size="sm"
                    class="h-8 w-8 p-0"
                  >
                    <MoreHorizontal class="h-4 w-4" />
                    <span class="sr-only">Open menu</span>
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem @click="emit('view', job)">
                    <ExternalLink class="mr-2 h-4 w-4" />
                    View Details
                  </DropdownMenuItem>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem
                    v-if="canPause(job)"
                    @click="emit('pause', job)"
                  >
                    <Pause class="mr-2 h-4 w-4" />
                    Pause
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    v-if="canResume(job)"
                    @click="emit('resume', job)"
                  >
                    <Play class="mr-2 h-4 w-4" />
                    Resume
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    v-if="canRunNow(job)"
                    @click="emit('runNow', job)"
                  >
                    <PlayCircle class="mr-2 h-4 w-4" />
                    Run now
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    v-if="canCancel(job)"
                    @click="emit('cancel', job)"
                  >
                    <XCircle class="mr-2 h-4 w-4" />
                    Cancel
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    v-if="canRetry(job)"
                    @click="emit('retry', job)"
                  >
                    <RotateCcw class="mr-2 h-4 w-4" />
                    Retry
                  </DropdownMenuItem>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem
                    class="text-destructive focus:text-destructive"
                    @click="emit('delete', job)"
                  >
                    <XCircle class="mr-2 h-4 w-4" />
                    Delete
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Pagination -->
    <div
      v-if="jobs.totalPages.value > 1 || jobs.totalJobs.value > 0"
      class="flex items-center justify-between border-t pt-4"
    >
      <p class="text-sm text-muted-foreground">
        Showing {{ (jobs.page.value - 1) * jobs.pageSize.value + 1 }} to
        {{ Math.min(jobs.page.value * jobs.pageSize.value, jobs.totalJobs.value) }}
        of {{ jobs.totalJobs.value }} jobs
      </p>

      <div class="flex items-center gap-4">
        <!-- Page Size Selector -->
        <div class="flex items-center gap-2">
          <span class="text-sm text-muted-foreground">Show:</span>
          <select
            :value="jobs.pageSize.value"
            class="rounded-md border bg-background px-2 py-1 text-sm"
            @change="handlePageSizeChange"
          >
            <option
              v-for="size in jobs.allowedPageSizes"
              :key="size"
              :value="size"
            >
              {{ size }}
            </option>
          </select>
        </div>

        <!-- Page Numbers -->
        <div class="flex items-center gap-1">
          <Button
            variant="outline"
            size="sm"
            :disabled="jobs.page.value === 1"
            @click="goToPage(jobs.page.value - 1)"
          >
            <ChevronLeft class="h-4 w-4" />
          </Button>

          <template
            v-for="page in pageNumbers"
            :key="page"
          >
            <Button
              v-if="typeof page === 'number'"
              :variant="page === jobs.page.value ? 'default' : 'outline'"
              size="sm"
              class="min-w-9"
              @click="goToPage(page)"
            >
              {{ page }}
            </Button>
            <span
              v-else
              class="px-2 text-muted-foreground"
            >...</span>
          </template>

          <Button
            variant="outline"
            size="sm"
            :disabled="jobs.page.value === jobs.totalPages.value"
            @click="goToPage(jobs.page.value + 1)"
          >
            <ChevronRight class="h-4 w-4" />
          </Button>
        </div>
      </div>
    </div>
  </div>
</template>
