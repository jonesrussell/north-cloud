<script setup lang="ts">
import Sidebar from './Sidebar.vue'
import ToastContainer from '../shared/components/ToastContainer.vue'
</script>

<template>
  <div class="flex min-h-screen bg-slate-950 text-slate-100">
    <Sidebar />
    <main class="flex-1 p-6 overflow-auto">
      <RouterView />
    </main>
    <ToastContainer />
  </div>
</template>
