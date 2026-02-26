<template>
  <div class="min-h-screen flex flex-col">
    <header class="sticky top-0 z-30 bg-[var(--nc-bg-elevated)]/95 backdrop-blur-sm border-b border-[var(--nc-border)]">
      <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div class="flex items-center gap-4 h-14 sm:h-16">
          <router-link
            to="/"
            class="flex-shrink-0 flex items-center gap-2 text-[var(--nc-text)] hover:text-[var(--nc-primary)] transition-colors duration-[var(--nc-duration)]"
            aria-label="North Cloud — Home"
          >
            <span class="font-display text-xl sm:text-2xl font-normal tracking-tight">
              North Cloud
            </span>
          </router-link>

          <form
            class="flex-1 flex justify-center"
            @submit.prevent="onSearch"
          >
            <div class="relative w-full max-w-lg">
              <svg
                class="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-[var(--nc-text-muted)] pointer-events-none"
                xmlns="http://www.w3.org/2000/svg"
                viewBox="0 0 20 20"
                fill="currentColor"
                aria-hidden="true"
              >
                <path
                  fill-rule="evenodd"
                  d="M9 3.5a5.5 5.5 0 1 0 0 11 5.5 5.5 0 0 0 0-11ZM2 9a7 7 0 1 1 12.452 4.391l3.328 3.329a.75.75 0 1 1-1.06 1.06l-3.329-3.328A7 7 0 0 1 2 9Z"
                  clip-rule="evenodd"
                />
              </svg>
              <input
                v-model="headerQuery"
                type="search"
                placeholder="Search..."
                class="w-full rounded-full bg-[var(--nc-bg-muted)] border border-[var(--nc-border)] py-1.5 pl-9 pr-4 text-sm text-[var(--nc-text)] placeholder:text-[var(--nc-text-muted)] focus:border-[var(--nc-primary)] focus:ring-2 focus:ring-[var(--nc-primary)]/25 focus:outline-none transition-colors duration-[var(--nc-duration)]"
              >
            </div>
          </form>
        </div>
      </div>
    </header>

    <main class="flex-1">
      <router-view />
    </main>

    <footer class="mt-auto border-t border-[var(--nc-border)] bg-[var(--nc-bg-elevated)]">
      <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
        <p class="text-center text-sm text-[var(--nc-text-muted)]">
          &copy; {{ currentYear }} North Cloud. All rights reserved.
        </p>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'

const router = useRouter()

const currentYear = computed(() => new Date().getFullYear())

const headerQuery = ref('')

function onSearch(): void {
  const trimmed = headerQuery.value.trim()
  if (!trimmed) return

  router.push({ path: '/search', query: { q: trimmed } })
  headerQuery.value = ''
}
</script>
