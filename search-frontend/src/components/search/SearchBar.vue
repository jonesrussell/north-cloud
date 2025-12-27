<template>
  <div class="relative">
    <div class="relative">
      <input
        v-model="localQuery"
        type="text"
        class="w-full px-4 py-3 pl-12 pr-12 text-lg border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
        @keydown.enter="handleSearch"
      />
      <div class="absolute inset-y-0 left-0 pl-4 flex items-center pointer-events-none">
        <svg class="h-5 w-5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
        </svg>
      </div>
      <button
        v-if="localQuery"
        @click="clearSearch"
        class="absolute inset-y-0 right-0 pr-4 flex items-center text-gray-400 hover:text-gray-600"
      >
        <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'

interface Props {
  modelValue?: string
}

const props = withDefaults(defineProps<Props>(), {
  modelValue: '',
})

const emit = defineEmits<{
  'update:modelValue': [value: string]
  'search': [query: string]
}>()

const localQuery = ref(props.modelValue)

watch(() => props.modelValue, (newValue: string) => {
  localQuery.value = newValue
})

watch(localQuery, (newValue: string) => {
  emit('update:modelValue', newValue)
})

const handleSearch = (): void => {
  if (localQuery.value.trim()) {
    emit('search', localQuery.value.trim())
  }
}

const clearSearch = (): void => {
  localQuery.value = ''
  emit('update:modelValue', '')
}
</script>
