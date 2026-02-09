<template>
  <div class="flex gap-2" role="search" aria-label="Search">
    <div ref="wrapperRef" class="relative flex-1">
      <input
        ref="inputRef"
        v-model="localQuery"
        type="search"
        autocomplete="off"
        class="w-full px-4 py-3 pl-12 pr-10 text-lg border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
        aria-label="Search query"
        aria-autocomplete="list"
        :aria-expanded="showDropdown"
        :aria-controls="showDropdown ? 'search-suggestions' : undefined"
        placeholder="Search..."
        @keydown="onKeydown"
        @focus="onFocus"
        @blur="onBlur"
      />
      <div class="absolute inset-y-0 left-0 pl-4 flex items-center pointer-events-none">
        <svg class="h-5 w-5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
        </svg>
      </div>
      <button
        v-if="localQuery"
        type="button"
        @click="clearSearch"
        class="absolute inset-y-0 right-0 pr-3 flex items-center text-gray-400 hover:text-gray-600"
        aria-label="Clear search"
      >
        <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>

      <!-- Suggest dropdown -->
      <div
        v-if="showDropdown && dropdownItems.length > 0"
        id="search-suggestions"
        role="listbox"
        aria-label="Search suggestions"
        class="absolute z-50 mt-1 w-full rounded-lg border border-gray-200 bg-white shadow-lg max-h-80 overflow-y-auto"
      >
        <template v-if="apiSuggestions.length > 0">
          <div class="px-3 py-2 text-xs font-medium text-gray-500 uppercase tracking-wider">
            Suggestions
          </div>
          <button
            v-for="(item, idx) in apiSuggestions"
            :key="`s-${item}`"
            type="button"
            role="option"
            :aria-selected="highlightedIndex === idx"
            :id="`suggest-option-${idx}`"
            class="w-full text-left px-4 py-2 text-sm text-gray-900 hover:bg-gray-100 focus:bg-gray-100 focus:outline-none"
            :class="{ 'bg-blue-50': highlightedIndex === idx }"
            @mousedown.prevent="selectItem(item)"
          >
            {{ item }}
          </button>
        </template>
        <template v-if="recentFiltered.length > 0">
          <div class="px-3 py-2 text-xs font-medium text-gray-500 uppercase tracking-wider border-t border-gray-100">
            Recent
          </div>
          <button
            v-for="(item, idx) in recentFiltered"
            :key="`r-${item}`"
            type="button"
            role="option"
            :aria-selected="highlightedIndex === apiSuggestions.length + idx"
            :id="`suggest-option-${apiSuggestions.length + idx}`"
            class="w-full text-left px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 focus:bg-gray-100 focus:outline-none"
            :class="{ 'bg-blue-50': highlightedIndex === apiSuggestions.length + idx }"
            @mousedown.prevent="selectItem(item)"
          >
            {{ item }}
          </button>
        </template>
      </div>
    </div>
    <button
      type="submit"
      class="px-6 py-3 text-lg font-medium text-white bg-blue-600 rounded-lg hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 shrink-0"
      @click="handleSearch"
    >
      Search
    </button>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, computed, nextTick, onMounted, onUnmounted } from 'vue'
import searchApi from '@/api/search'
import { useDebounce } from '@/composables/useDebounce'
import { getRecentSearches, addRecentSearch } from '@/composables/useRecentSearches'
import { trackEvent } from '@/utils/analytics'

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
const apiSuggestions = ref<string[]>([])
const recentSearches = ref<string[]>(getRecentSearches())
const showDropdown = ref(false)
const highlightedIndex = ref(0)
const wrapperRef = ref<HTMLElement | null>(null)
const inputRef = ref<HTMLInputElement | null>(null)

const debouncedQuery = useDebounce(localQuery, 280)

const dropdownItems = computed((): string[] => {
  return [...apiSuggestions.value, ...recentFiltered.value]
})

const recentFiltered = computed((): string[] => {
  const q = localQuery.value.trim().toLowerCase()
  const recent = recentSearches.value
  if (!q) return recent
  return recent.filter((r) => r.toLowerCase().startsWith(q) || r.toLowerCase().includes(q))
})

watch(() => props.modelValue, (newValue: string) => {
  localQuery.value = newValue
})

watch(localQuery, (newValue: string) => {
  emit('update:modelValue', newValue)
})

watch(debouncedQuery, async (q: string) => {
  const trimmed = q.trim()
  if (trimmed.length < 2) {
    apiSuggestions.value = []
    return
  }
  try {
    const res = await searchApi.suggest(trimmed)
    apiSuggestions.value = res.data.suggestions ?? []
  } catch {
    apiSuggestions.value = []
  }
})

watch(showDropdown, (open: boolean) => {
  if (open) {
    highlightedIndex.value = 0
  }
})

watch(dropdownItems, () => {
  highlightedIndex.value = 0
})

function onFocus(): void {
  recentSearches.value = getRecentSearches()
  if (dropdownItems.value.length > 0) {
    showDropdown.value = true
  }
}

function onBlur(): void {
  // Allow click on suggestion to fire before closing
  setTimeout(() => {
    showDropdown.value = false
  }, 180)
}

function selectItem(item: string): void {
  localQuery.value = item
  emit('update:modelValue', item)
  showDropdown.value = false
  trackEvent('search_suggestion_select', { suggestion: item, query: localQuery.value })
  emit('search', item.trim())
  addRecentSearch(item.trim())
}

function onKeydown(e: KeyboardEvent): void {
  if (!showDropdown.value || dropdownItems.value.length === 0) {
    if (e.key === 'Enter') {
      handleSearch()
    }
    return
  }
  if (e.key === 'Escape') {
    showDropdown.value = false
    e.preventDefault()
    return
  }
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    highlightedIndex.value = Math.min(highlightedIndex.value + 1, dropdownItems.value.length - 1)
    nextTick(() => updateActiveDescendant())
    return
  }
  if (e.key === 'ArrowUp') {
    e.preventDefault()
    highlightedIndex.value = Math.max(highlightedIndex.value - 1, 0)
    nextTick(() => updateActiveDescendant())
    return
  }
  if (e.key === 'Enter') {
    const item = dropdownItems.value[highlightedIndex.value]
    if (item) {
      e.preventDefault()
      selectItem(item)
    } else {
      handleSearch()
    }
    return
  }
}

function updateActiveDescendant(): void {
  const id = highlightedIndex.value < dropdownItems.value.length
    ? `suggest-option-${highlightedIndex.value}`
    : undefined
  const input = inputRef.value
  if (input) {
    if (id) input.setAttribute('aria-activedescendant', id)
    else input.removeAttribute('aria-activedescendant')
  }
}

function handleSearch(): void {
  if (localQuery.value.trim()) {
    trackEvent('search_submit', { query: localQuery.value.trim() })
    addRecentSearch(localQuery.value.trim())
    emit('search', localQuery.value.trim())
  }
  showDropdown.value = false
}

function clearSearch(): void {
  localQuery.value = ''
  emit('update:modelValue', '')
  showDropdown.value = false
}

// Click outside to close
function handleClickOutside(event: MouseEvent): void {
  if (wrapperRef.value && !wrapperRef.value.contains(event.target as Node)) {
    showDropdown.value = false
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})
onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>
