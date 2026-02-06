<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Loader2 } from 'lucide-vue-next'
import { useAuth } from '@/composables/useAuth'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

const router = useRouter()
const { login, isAuthenticated } = useAuth()

const username = ref('')
const password = ref('')
const loading = ref(false)
const error = ref<string | null>(null)

// Redirect if already authenticated
onMounted(() => {
  if (isAuthenticated.value) {
    router.push('/')
  }
})

const handleLogin = async () => {
  error.value = null
  loading.value = true

  try {
    const result = await login(username.value, password.value)

    if (result.success) {
      router.push('/')
    } else {
      error.value = result.error || 'Login failed'
    }
  } catch (err) {
    error.value = (err as Error).message || 'An unexpected error occurred'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-[hsl(220_14%_6%)] px-4 relative overflow-hidden">
    <!-- Animated grid background -->
    <div class="absolute inset-0 opacity-[0.03]">
      <div
        class="absolute inset-0"
        style="background-image: linear-gradient(hsl(185 80% 50%) 1px, transparent 1px), linear-gradient(90deg, hsl(185 80% 50%) 1px, transparent 1px); background-size: 60px 60px;"
      />
    </div>

    <!-- Subtle scan line effect -->
    <div class="absolute inset-0 pointer-events-none overflow-hidden opacity-[0.02]">
      <div
        class="absolute inset-0 h-[200%]"
        style="background: repeating-linear-gradient(0deg, transparent, transparent 2px, hsl(185 80% 50%) 2px, hsl(185 80% 50%) 4px); animation: scan-line 8s linear infinite;"
      />
    </div>

    <!-- Login card -->
    <div class="w-full max-w-sm relative animate-fade-up">
      <div class="border border-[hsl(220_13%_18%)] bg-[hsl(220_14%_9%)] rounded-sm shadow-2xl shadow-black/50">
        <!-- Header -->
        <div class="px-8 pt-10 pb-2 text-center">
          <!-- NC brand mark -->
          <div class="mx-auto mb-6 flex h-12 w-12 items-center justify-center rounded-sm border border-[hsl(185_80%_50%_/_0.3)] bg-[hsl(185_80%_50%_/_0.1)]">
            <span class="font-mono font-bold text-lg text-[hsl(185_80%_50%)]">NC</span>
          </div>
          <h1 class="font-mono text-sm font-semibold tracking-[0.2em] uppercase text-[hsl(210_20%_93%)]">
            North Cloud
          </h1>
          <p class="mt-1 text-xs text-[hsl(220_10%_45%)] font-mono">
            Content Intelligence Platform
          </p>
        </div>

        <!-- Form -->
        <div class="px-8 pb-8 pt-6">
          <form
            class="space-y-4"
            @submit.prevent="handleLogin"
          >
            <!-- Error message -->
            <div
              v-if="error"
              class="rounded-sm bg-[hsl(0_72%_51%_/_0.1)] border border-[hsl(0_72%_51%_/_0.2)] p-3 text-xs text-[hsl(0_72%_60%)] font-mono"
            >
              {{ error }}
            </div>

            <div class="space-y-1.5">
              <label
                for="username"
                class="text-[10px] font-mono font-medium uppercase tracking-widest text-[hsl(220_10%_45%)]"
              >Username</label>
              <Input
                id="username"
                v-model="username"
                type="text"
                placeholder="admin"
                :disabled="loading"
                class="bg-[hsl(220_14%_7%)] border-[hsl(220_13%_18%)] text-[hsl(210_20%_93%)] placeholder:text-[hsl(220_10%_30%)] font-mono"
                required
              />
            </div>

            <div class="space-y-1.5">
              <label
                for="password"
                class="text-[10px] font-mono font-medium uppercase tracking-widest text-[hsl(220_10%_45%)]"
              >Password</label>
              <Input
                id="password"
                v-model="password"
                type="password"
                placeholder="••••••••"
                :disabled="loading"
                class="bg-[hsl(220_14%_7%)] border-[hsl(220_13%_18%)] text-[hsl(210_20%_93%)] placeholder:text-[hsl(220_10%_30%)] font-mono"
                required
              />
            </div>

            <Button
              type="submit"
              class="w-full font-mono text-xs tracking-wider uppercase mt-6"
              :disabled="loading"
            >
              <Loader2
                v-if="loading"
                class="mr-2 h-3.5 w-3.5 animate-spin"
              />
              {{ loading ? 'Authenticating...' : 'Sign In' }}
            </Button>
          </form>
        </div>
      </div>

      <!-- Footer -->
      <p class="mt-6 text-center text-[10px] text-[hsl(220_10%_30%)] font-mono tracking-wider">
        v2.0 &middot; Content Pipeline
      </p>
    </div>
  </div>
</template>
