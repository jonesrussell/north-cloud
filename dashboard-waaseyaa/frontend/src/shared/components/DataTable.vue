<script setup lang="ts">
import { computed } from 'vue'

export interface Column {
  key: string
  label: string
  sortable?: boolean
}

const props = defineProps<{
  columns: Column[]
  rows: Record<string, unknown>[]
  loading?: boolean
  total?: number
  sortKey?: string
  sortDir?: 'asc' | 'desc'
}>()

const emit = defineEmits<{
  sort: [key: string]
  'page-change': [page: number]
}>()

function handleSort(key: string) {
  emit('sort', key)
}
</script>

<template>
  <div class="overflow-x-auto">
    <table class="w-full text-sm text-left">
      <thead class="text-xs uppercase text-slate-400 border-b border-slate-800">
        <tr>
          <th
            v-for="col in columns"
            :key="col.key"
            class="px-4 py-3"
            :class="col.sortable ? 'cursor-pointer hover:text-slate-200' : ''"
            @click="col.sortable && handleSort(col.key)"
          >
            {{ col.label }}
            <span v-if="col.sortable && sortKey === col.key" class="ml-1">
              {{ sortDir === 'asc' ? '↑' : '↓' }}
            </span>
          </th>
        </tr>
      </thead>
      <tbody v-if="loading">
        <tr v-for="i in 5" :key="i">
          <td v-for="col in columns" :key="col.key" class="px-4 py-3">
            <div class="h-4 bg-slate-800 rounded animate-pulse" />
          </td>
        </tr>
      </tbody>
      <tbody v-else>
        <tr
          v-for="(row, idx) in rows"
          :key="idx"
          class="border-b border-slate-800/50 hover:bg-slate-900/50"
        >
          <td v-for="col in columns" :key="col.key" class="px-4 py-3 text-slate-300">
            <slot :name="col.key" :row="row" :value="row[col.key]">
              {{ row[col.key] }}
            </slot>
          </td>
        </tr>
        <tr v-if="rows.length === 0">
          <td :colspan="columns.length" class="px-4 py-8 text-center text-slate-500">
            No data available
          </td>
        </tr>
      </tbody>
    </table>
    <div v-if="total" class="px-4 py-2 text-xs text-slate-500 border-t border-slate-800">
      {{ total }} total items
    </div>
  </div>
</template>
