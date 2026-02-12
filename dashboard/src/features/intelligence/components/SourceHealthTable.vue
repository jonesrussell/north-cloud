<script setup lang="ts">
import { ref, computed } from 'vue'
import type { SourceMetrics } from '../problems/types'
import Badge from '@/components/ui/badge/Badge.vue'

const props = defineProps<{
  sources: SourceMetrics[]
}>()

type ViewMode = 'ops' | 'dev'
type QuickFilter = 'all' | 'errors' | 'warnings' | 'no-docs' | 'backlog'

const BACKLOG_WARNING_THRESHOLD = 100

const viewMode = ref<ViewMode>(
  (localStorage.getItem('intelligence-view-mode') as ViewMode) ?? 'ops',
)
const quickFilter = ref<QuickFilter>('all')

function setViewMode(mode: ViewMode) {
  viewMode.value = mode
  localStorage.setItem('intelligence-view-mode', mode)
}

function getStatus(source: SourceMetrics): 'error' | 'warning' | 'healthy' {
  if (!source.active) return 'healthy'
  if (source.classifiedCount === 0) return 'error'
  if (source.backlog > BACKLOG_WARNING_THRESHOLD) return 'warning'
  if (source.delta24h === 0 && source.classifiedCount > 0) return 'warning'
  return 'healthy'
}

const filteredSources = computed(() => {
  let result = [...props.sources]

  switch (quickFilter.value) {
    case 'errors':
      result = result.filter((s) => getStatus(s) === 'error')
      break
    case 'warnings':
      result = result.filter((s) => getStatus(s) === 'warning')
      break
    case 'no-docs':
      result = result.filter((s) => s.active && s.classifiedCount === 0)
      break
    case 'backlog':
      result = result.filter((s) => s.backlog > 0)
      break
  }

  result.sort((a, b) => {
    const order = { error: 0, warning: 1, healthy: 2 }
    return order[getStatus(a)] - order[getStatus(b)]
  })

  return result
})

const filters: { key: QuickFilter; label: string }[] = [
  { key: 'all', label: 'All' },
  { key: 'errors', label: 'Errors' },
  { key: 'warnings', label: 'Warnings' },
  { key: 'no-docs', label: 'No docs' },
  { key: 'backlog', label: 'Backlog' },
]

const statusDot: Record<string, string> = {
  error: 'bg-red-500',
  warning: 'bg-amber-500',
  healthy: 'bg-emerald-500',
}
</script>

<template>
  <div class="space-y-3">
    <!-- Controls -->
    <div class="flex items-center justify-between gap-3">
      <div class="flex gap-1.5">
        <button
          v-for="f in filters"
          :key="f.key"
          class="rounded-md px-2.5 py-1 text-xs font-medium transition-colors"
          :class="
            quickFilter === f.key
              ? 'bg-primary text-primary-foreground'
              : 'bg-muted text-muted-foreground hover:bg-muted/80'
          "
          @click="quickFilter = f.key"
        >
          {{ f.label }}
        </button>
      </div>
      <div class="flex gap-1.5">
        <button
          class="rounded-md px-2.5 py-1 text-xs font-medium transition-colors"
          :class="
            viewMode === 'ops'
              ? 'bg-primary text-primary-foreground'
              : 'bg-muted text-muted-foreground'
          "
          @click="setViewMode('ops')"
        >
          Ops
        </button>
        <button
          class="rounded-md px-2.5 py-1 text-xs font-medium transition-colors"
          :class="
            viewMode === 'dev'
              ? 'bg-primary text-primary-foreground'
              : 'bg-muted text-muted-foreground'
          "
          @click="setViewMode('dev')"
        >
          Dev
        </button>
      </div>
    </div>

    <!-- Table -->
    <div class="rounded-lg border overflow-hidden">
      <table class="w-full text-sm">
        <thead class="border-b bg-muted/50">
          <tr>
            <th class="px-3 py-2 text-left font-medium text-muted-foreground">
              Source
            </th>
            <th class="px-3 py-2 text-left font-medium text-muted-foreground">
              Status
            </th>
            <template v-if="viewMode === 'ops'">
              <th class="px-3 py-2 text-right font-medium text-muted-foreground">
                Raw
              </th>
              <th class="px-3 py-2 text-right font-medium text-muted-foreground">
                Classified
              </th>
              <th class="px-3 py-2 text-right font-medium text-muted-foreground">
                Backlog
              </th>
            </template>
            <template v-else>
              <th class="px-3 py-2 text-right font-medium text-muted-foreground">
                Classified
              </th>
              <th class="px-3 py-2 text-right font-medium text-muted-foreground">
                Avg Quality
              </th>
              <th class="px-3 py-2 text-right font-medium text-muted-foreground">
                24h Delta
              </th>
            </template>
          </tr>
        </thead>
        <tbody class="divide-y">
          <tr
            v-for="source in filteredSources"
            :key="source.source"
            :class="{ 'opacity-50': !source.active }"
          >
            <td class="px-3 py-2 font-mono text-xs">
              {{ source.source.replaceAll('_', '.') }}
            </td>
            <td class="px-3 py-2">
              <span
                class="inline-block h-2 w-2 rounded-full"
                :class="statusDot[getStatus(source)]"
              />
            </td>
            <template v-if="viewMode === 'ops'">
              <td class="px-3 py-2 text-right tabular-nums">
                {{ source.rawCount.toLocaleString() }}
              </td>
              <td class="px-3 py-2 text-right tabular-nums">
                {{ source.classifiedCount.toLocaleString() }}
              </td>
              <td
                class="px-3 py-2 text-right tabular-nums"
                :class="{ 'text-amber-500': source.backlog > 0 }"
              >
                {{ source.backlog > 0 ? source.backlog.toLocaleString() : '-' }}
              </td>
            </template>
            <template v-else>
              <td class="px-3 py-2 text-right tabular-nums">
                {{ source.classifiedCount.toLocaleString() }}
              </td>
              <td class="px-3 py-2 text-right tabular-nums">
                <Badge
                  v-if="source.avgQuality > 0"
                  :variant="
                    source.avgQuality >= 70
                      ? 'success'
                      : source.avgQuality >= 40
                        ? 'warning'
                        : 'destructive'
                  "
                >
                  {{ Math.round(source.avgQuality) }}
                </Badge>
                <span
                  v-else
                  class="text-muted-foreground"
                >-</span>
              </td>
              <td
                class="px-3 py-2 text-right tabular-nums"
                :class="{
                  'text-amber-500': source.delta24h === 0 && source.classifiedCount > 0,
                }"
              >
                {{
                  source.delta24h > 0
                    ? `+${source.delta24h.toLocaleString()}`
                    : source.delta24h === 0 && source.classifiedCount > 0
                      ? 'stale'
                      : '-'
                }}
              </td>
            </template>
          </tr>
          <tr v-if="filteredSources.length === 0">
            <td
              :colspan="5"
              class="px-3 py-8 text-center text-sm text-muted-foreground"
            >
              No sources match the current filter.
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <p class="text-xs text-muted-foreground">
      {{ filteredSources.length }} of {{ sources.length }} sources
    </p>
  </div>
</template>
