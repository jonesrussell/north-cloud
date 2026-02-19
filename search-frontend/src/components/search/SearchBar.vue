<template>
  <div
    class="flex gap-2 sm:gap-3"
    role="search"
    aria-label="Search"
  >
    <div
      ref="wrapperRef"
      class="relative flex-1"
    >
      <input
        ref="inputRef"
        v-model="localQuery"
        type="search"
        autocomplete="off"
        class="search-input w-full px-4 py-3.5 pl-12 pr-11 text-base sm:text-lg border border-[var(--nc-border)] rounded-xl bg-[var(--nc-bg-elevated)] text-[var(--nc-text)] placeholder-[var(--nc-text-muted)] focus:outline-none focus:ring-2 focus:ring-[var(--nc-primary)] focus:border-transparent shadow-[var(--nc-shadow-sm)] transition-shadow duration-[var(--nc-duration)]"
        aria-label="Search query"
        aria-autocomplete="list"
        :aria-expanded="showDropdown"
        :aria-controls="showDropdown ? 'search-suggestions' : undefined"
        placeholder="Search..."
        @keydown="onKeydown"
        @focus="onFocus"
        @blur="onBlur"
      >
      <div class="absolute inset-y-0 left-0 pl-4 flex items-center pointer-events-none text-[var(--nc-text-muted)]">
        <svg
          class="h-5 w-5"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          stroke-width="1.8"
          aria-hidden="true"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
          />
        </svg>
      </div>
      <button
        v-if="localQuery"
        type="button"
        class="absolute inset-y-0 right-0 pr-3 flex items-center text-[var(--nc-text-muted)] hover:text-[var(--nc-text)] transition-colors duration-[var(--nc-duration)]"
        aria-label="Clear search"
        @click="clearSearch"
      >
        <svg
          class="h-5 w-5"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          stroke-width="2"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            d="M6 18L18 6M6 6l12 12"
          />
        </svg>
      </button>

      <!-- Suggestions dropdown -->
      <div
        v-if="showDropdown && dropdownItems.length > 0"
        id="search-suggestions"
        role="listbox"
        aria-label="Search suggestions"
        class="absolute z-50 mt-2 w-full rounded-xl border border-[var(--nc-border)] bg-[var(--nc-bg-elevated)] shadow-[var(--nc-shadow-lg)] max-h-80 overflow-y-auto overflow-x-hidden"
      >
        <template v-if="apiSuggestions.length > 0">
          <div class="px-4 py-2.5 text-xs font-semibold text-[var(--nc-text-muted)] uppercase tracking-wider">
            Suggestions
          </div>
          <button
            v-for="(item, idx) in apiSuggestions"
            :id="`suggest-option-${idx}`"
            :key="`s-${item}`"
            type="button"
            role="option"
            :aria-selected="highlightedIndex === idx"
            class="w-full text-left px-4 py-2.5 text-sm text-[var(--nc-text)] hover:bg-[var(--nc-primary-muted)] focus:bg-[var(--nc-primary-muted)] focus:outline-none transition-colors duration-[var(--nc-duration-fast)]"
            :class="{ 'bg-[var(--nc-primary-muted)]': highlightedIndex === idx }"
            @mousedown.prevent="selectItem(item)"
          >
            {{ item }}
          </button>
        </template>
        <template v-if="recentFiltered.length > 0">
          <div class="px-4 py-2.5 text-xs font-semibold text-[var(--nc-text-muted)] uppercase tracking-wider border-t border-[var(--nc-border)]">
            Recent
          </div>
          <button
            v-for="(item, idx) in recentFiltered"
            :id="`suggest-option-${apiSuggestions.length + idx}`"
            :key="`r-${item}`"
            type="button"
            role="option"
            :aria-selected="highlightedIndex === apiSuggestions.length + idx"
            class="w-full text-left px-4 py-2.5 text-sm text-[var(--nc-text-secondary)] hover:bg-[var(--nc-primary-muted)] focus:bg-[var(--nc-primary-muted)] focus:outline-none transition-colors duration-[var(--nc-duration-fast)]"
            :class="{ 'bg-[var(--nc-primary-muted)]': highlightedIndex === apiSuggestions.length + idx }"
            @mousedown.prevent="selectItem(item)"
          >
            {{ item }}
          </button>
        </template>
      </div>
    </div>
    <button
      type="submit"
      class="search-btn px-5 sm:px-6 py-3.5 text-base font-semibold text-white rounded-xl bg-[var(--nc-accent)] hover:bg-[var(--nc-accent-hover)] focus:outline-none focus:ring-2 focus:ring-[var(--nc-accent)] focus:ring-offset-2 shadow-[var(--nc-shadow-sm)] transition-colors duration-[var(--nc-duration)] shrink-0"
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
