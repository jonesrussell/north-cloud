<template>
  <div class="min-h-screen bg-background">
    <!-- Authenticated layout -->
    <template v-if="isAuthenticated">
      <AppShell>
        <router-view />
      </AppShell>
    </template>

    <!-- Public layout (login page) -->
    <template v-else>
      <router-view />
    </template>
  </div>
</template>

<script setup lang="ts">
import { useAuth } from './composables/useAuth'
import { useTheme } from './composables/useTheme'
import { AppShell } from './components/layout'

const { isAuthenticated } = useAuth()

// Initialize theme on app mount
useTheme()
</script>
