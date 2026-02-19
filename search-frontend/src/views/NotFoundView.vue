<template>
  <div class="min-h-[60vh] flex items-center justify-center px-4">
    <div class="text-center">
      <p class="font-display text-6xl sm:text-7xl font-normal text-[var(--nc-text-muted)] mb-4">
        404
      </p>
      <p class="text-xl text-[var(--nc-text-secondary)] mb-8">
        Page not found
      </p>
      <router-link
        to="/"
        class="inline-flex items-center rounded-lg border border-transparent bg-[var(--nc-accent)] px-4 py-2 text-base font-medium text-white hover:bg-[var(--nc-accent-hover)] focus:outline-none focus:ring-2 focus:ring-[var(--nc-accent)] focus:ring-offset-2 transition-colors duration-[var(--nc-duration)]"
      >
        Go back home
      </router-link>
    </div>
  </div>
</template>

<script setup lang="ts">
// 404 Not Found page
</script>
