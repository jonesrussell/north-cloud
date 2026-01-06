<script setup lang="ts">
import { useRouter } from 'vue-router'
import { Home, ArrowLeft } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'

const router = useRouter()
</script>

<template>
  <div class="min-h-[80vh] flex flex-col items-center justify-center text-center px-4">
    <div class="text-9xl font-bold text-muted-foreground/20">404</div>
    <h1 class="mt-4 text-3xl font-bold tracking-tight">Page not found</h1>
    <p class="mt-2 text-muted-foreground max-w-md">
      Sorry, we couldn't find the page you're looking for. It may have been moved or doesn't exist.
    </p>
    <div class="mt-8 flex gap-4">
      <Button variant="outline" @click="router.back()">
        <ArrowLeft class="mr-2 h-4 w-4" />
        Go back
      </Button>
      <Button @click="router.push('/')">
        <Home class="mr-2 h-4 w-4" />
        Home
      </Button>
    </div>
  </div>
</template>
