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

    <!-- Global toast notifications -->
    <Toaster
      position="bottom-right"
      :rich-colors="true"
      :close-button="true"
    />
  </div>
</template>

<script setup lang="ts">
import { useAuth } from './composables/useAuth'
import { useTheme } from './composables/useTheme'
import { AppShell } from './components/layout'
import { Toaster } from 'vue-sonner'

const { isAuthenticated } = useAuth()

// Initialize theme on app mount
useTheme()
</script>
