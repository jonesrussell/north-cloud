<script setup lang="ts">
import {
  Database,
  FileText,
  HardDrive,
  Activity,
} from 'lucide-vue-next'
import { Skeleton } from '@/components/ui/skeleton'
import type { IndexStats } from '@/types/indexManager'

interface Props {
  stats: IndexStats | undefined
  loading?: boolean
}

withDefaults(defineProps<Props>(), {
  loading: false,
})

function formatNumber(num: number | undefined): string {
  if (num === undefined) return '0'
  if (num >= 1000000) return (num / 1000000).toFixed(1) + 'M'
  if (num >= 1000) return (num / 1000).toFixed(1) + 'K'
  return num.toLocaleString()
}

</script>

<template>
  <div class="grid grid-cols-2 gap-4 md:grid-cols-4">
    <!-- Total Indexes -->
    <div class="rounded-lg border bg-card p-4">
      <div class="flex items-center gap-3">
        <div class="rounded-lg bg-blue-500/10 p-2">
          <Database class="h-5 w-5 text-blue-500" />
        </div>
        <div>
          <Skeleton
            v-if="loading"
            class="mb-1 h-7 w-12"
          />
          <div
            v-else
            class="text-2xl font-bold tabular-nums"
          >
            {{ stats?.total_indexes ?? 0 }}
          </div>
          <div class="text-xs text-muted-foreground uppercase tracking-wider">
            Indexes
          </div>
        </div>
      </div>
    </div>

    <!-- Total Documents -->
    <div class="rounded-lg border bg-card p-4">
      <div class="flex items-center gap-3">
        <div class="rounded-lg bg-violet-500/10 p-2">
          <FileText class="h-5 w-5 text-violet-500" />
        </div>
        <div>
          <Skeleton
            v-if="loading"
            class="mb-1 h-7 w-16"
          />
          <div
            v-else
            class="text-2xl font-bold tabular-nums"
          >
            {{ formatNumber(stats?.total_documents) }}
          </div>
          <div class="text-xs text-muted-foreground uppercase tracking-wider">
            Documents
          </div>
        </div>
      </div>
    </div>

    <!-- Indexed Today -->
    <div class="rounded-lg border bg-card p-4">
      <div class="flex items-center gap-3">
        <div class="rounded-lg bg-cyan-500/10 p-2">
          <HardDrive class="h-5 w-5 text-cyan-500" />
        </div>
        <div>
          <Skeleton
            v-if="loading"
            class="mb-1 h-7 w-16"
          />
          <div
            v-else
            class="text-2xl font-bold tabular-nums"
          >
            {{ formatNumber(stats?.indexed_today) }}
          </div>
          <div class="text-xs text-muted-foreground uppercase tracking-wider">
            Today
          </div>
        </div>
      </div>
    </div>

    <!-- Health Status -->
    <div class="rounded-lg border bg-card p-4">
      <div class="flex items-center gap-3">
        <div class="rounded-lg bg-emerald-500/10 p-2">
          <Activity class="h-5 w-5 text-emerald-500" />
        </div>
        <div>
          <Skeleton
            v-if="loading"
            class="mb-1 h-7 w-24"
          />
          <div
            v-else
            class="flex items-center gap-2"
          >
            <div class="flex items-center gap-1">
              <span class="inline-block h-2 w-2 rounded-full bg-emerald-400" />
              <span class="text-sm font-semibold text-emerald-500">
                {{ stats?.indexes_by_health?.green ?? 0 }}
              </span>
            </div>
            <div class="flex items-center gap-1">
              <span class="inline-block h-2 w-2 rounded-full bg-amber-400" />
              <span class="text-sm font-semibold text-amber-500">
                {{ stats?.indexes_by_health?.yellow ?? 0 }}
              </span>
            </div>
            <div class="flex items-center gap-1">
              <span class="inline-block h-2 w-2 rounded-full bg-rose-400" />
              <span class="text-sm font-semibold text-rose-500">
                {{ stats?.indexes_by_health?.red ?? 0 }}
              </span>
            </div>
          </div>
          <div class="text-xs text-muted-foreground uppercase tracking-wider">
            Health Status
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
