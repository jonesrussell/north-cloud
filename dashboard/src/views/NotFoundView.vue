<script setup lang="ts">
import { useRouter } from 'vue-router'
import { Home, ArrowLeft } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'

const router = useRouter()
</script>

<template>
  <div class="min-h-[80vh] flex flex-col items-center justify-center text-center px-4 animate-fade-up">
    <!-- Giant 404 with noise effect -->
    <div class="relative select-none">
      <span class="text-[10rem] leading-none font-mono font-bold text-muted-foreground/5 tracking-tighter">
        404
      </span>
      <span
        class="absolute inset-0 text-[10rem] leading-none font-mono font-bold text-muted-foreground/10 tracking-tighter"
        style="clip-path: inset(0 0 50% 0); transform: translateX(2px);"
      >
        404
      </span>
    </div>
    <h1 class="mt-2 text-lg font-mono font-medium tracking-tight uppercase">
      Page not found
    </h1>
    <p class="mt-2 text-sm text-muted-foreground max-w-sm font-mono">
      The requested resource does not exist or has been relocated.
    </p>
    <div class="mt-8 flex gap-3">
      <Button
        variant="outline"
        size="sm"
        class="font-mono"
        @click="router.back()"
      >
        <ArrowLeft class="mr-2 h-3.5 w-3.5" />
        Go back
      </Button>
      <Button
        size="sm"
        class="font-mono"
        @click="router.push('/')"
      >
        <Home class="mr-2 h-3.5 w-3.5" />
        Home
      </Button>
    </div>
  </div>
</template>
