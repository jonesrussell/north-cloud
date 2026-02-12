<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { AlertTriangle, XCircle } from 'lucide-vue-next'
import type { Problem } from './types'

const props = defineProps<{
  problems: Problem[]
}>()

const router = useRouter()

const errors = computed(() => props.problems.filter((p) => p.severity === 'error'))

function handleClick(problem: Problem) {
  if (problem.link) {
    router.push(problem.link)
  }
}
</script>

<template>
  <div
    v-if="problems.length > 0"
    class="rounded-lg border p-4 space-y-2"
    :class="errors.length > 0 ? 'border-red-500/30 bg-red-500/5' : 'border-amber-500/30 bg-amber-500/5'"
  >
    <div class="flex items-center gap-2 text-sm font-medium">
      <XCircle
        v-if="errors.length > 0"
        class="h-4 w-4 text-red-500 shrink-0"
      />
      <AlertTriangle
        v-else
        class="h-4 w-4 text-amber-500 shrink-0"
      />
      <span>
        {{ problems.length }} issue{{ problems.length === 1 ? '' : 's' }} detected
      </span>
    </div>
    <div class="flex flex-wrap gap-2">
      <button
        v-for="problem in problems"
        :key="problem.id"
        class="inline-flex items-center gap-1.5 rounded-md px-2.5 py-1 text-xs font-medium transition-colors"
        :class="
          problem.severity === 'error'
            ? 'bg-red-500/10 text-red-700 dark:text-red-400 hover:bg-red-500/20'
            : 'bg-amber-500/10 text-amber-700 dark:text-amber-400 hover:bg-amber-500/20'
        "
        :title="problem.action"
        @click="handleClick(problem)"
      >
        <span
          v-if="problem.count"
          class="font-semibold tabular-nums"
        >{{ problem.count }}</span>
        {{ problem.title }}
      </button>
    </div>
  </div>
</template>
