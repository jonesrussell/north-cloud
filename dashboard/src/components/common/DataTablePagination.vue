<script setup lang="ts">
import { computed } from 'vue'
import { ChevronLeft, ChevronRight } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { usePageNumbers } from '@/composables/usePageNumbers'

const props = withDefaults(
  defineProps<{
    page: number
    pageSize: number
    total: number
    totalPages: number
    allowedPageSizes: readonly number[]
    itemLabel?: string
  }>(),
  {
    itemLabel: 'items',
  }
)

const emit = defineEmits<{
  (e: 'update:page', value: number): void
  (e: 'update:pageSize', value: number): void
}>()

const pageRef = computed(() => props.page)
const totalPagesRef = computed(() => props.totalPages)
const { pageNumbers } = usePageNumbers(pageRef, totalPagesRef)

const startItem = computed(() =>
  props.total === 0 ? 0 : (props.page - 1) * props.pageSize + 1
)
const endItem = computed(() =>
  Math.min(props.page * props.pageSize, props.total)
)

function goToPage(page: number | string) {
  if (typeof page === 'number') {
    emit('update:page', page)
  }
}

function handlePageSizeChange(event: Event) {
  const target = event.target as HTMLSelectElement
  emit('update:pageSize', Number(target.value))
}
</script>

<template>
  <div
    v-if="totalPages > 1 || total > 0"
    class="flex items-center justify-between border-t pt-4"
  >
    <p class="text-sm text-muted-foreground">
      Showing {{ startItem }} to {{ endItem }} of {{ total }} {{ itemLabel }}
    </p>

    <div class="flex items-center gap-4">
      <!-- Page Size Selector -->
      <div class="flex items-center gap-2">
        <span class="text-sm text-muted-foreground">Show:</span>
        <select
          :value="pageSize"
          class="rounded-md border bg-background px-2 py-1 text-sm"
          @change="handlePageSizeChange"
        >
          <option
            v-for="size in allowedPageSizes"
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
          :disabled="page === 1"
          @click="goToPage(page - 1)"
        >
          <ChevronLeft class="h-4 w-4" />
        </Button>

        <template
          v-for="p in pageNumbers"
          :key="String(p)"
        >
          <Button
            v-if="typeof p === 'number'"
            :variant="p === page ? 'default' : 'outline'"
            size="sm"
            class="min-w-9"
            @click="goToPage(p)"
          >
            {{ p }}
          </Button>
          <span
            v-else
            class="px-2 text-muted-foreground"
          >...</span>
        </template>

        <Button
          variant="outline"
          size="sm"
          :disabled="page >= totalPages"
          @click="goToPage(page + 1)"
        >
          <ChevronRight class="h-4 w-4" />
        </Button>
      </div>
    </div>
  </div>
</template>
