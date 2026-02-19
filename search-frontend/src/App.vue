<template>
  <div class="min-h-screen flex flex-col">
    <header class="sticky top-0 z-30 bg-[var(--nc-bg-elevated)]/95 backdrop-blur-sm border-b border-[var(--nc-border)]">
      <div class="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8">
        <div class="flex justify-between items-center h-14 sm:h-16">
          <router-link
            v-if="showHeaderLogo"
            to="/"
            class="flex items-center gap-2 text-[var(--nc-text)] hover:text-[var(--nc-primary)] transition-colors duration-[var(--nc-duration)]"
            aria-label="North Cloud — Home"
          >
            <span class="font-display text-xl sm:text-2xl font-normal tracking-tight">
              North Cloud
            </span>
          </router-link>
          <span
            v-else
            class="h-4"
            aria-hidden="true"
          />
          <nav class="flex items-center gap-4">
            <router-link
              to="/advanced"
              class="text-sm font-medium text-[var(--nc-text-secondary)] hover:text-[var(--nc-primary)] transition-colors duration-[var(--nc-duration)]"
            >
              Advanced search
            </router-link>
          </nav>
        </div>
      </div>
    </header>

    <main class="flex-1">
      <router-view />
    </main>

    <footer class="mt-auto border-t border-[var(--nc-border)] bg-[var(--nc-bg-elevated)]">
      <div class="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
        <p class="text-center text-sm text-[var(--nc-text-muted)]">
          &copy; {{ currentYear }} North Cloud. All rights reserved.
        </p>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

const currentYear = computed(() => new Date().getFullYear())

const route = useRoute()
const showHeaderLogo = computed(() => route.name !== 'home')
</script>
