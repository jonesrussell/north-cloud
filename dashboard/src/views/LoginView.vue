<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { CloudCog, Loader2 } from 'lucide-vue-next'
import { useAuth } from '@/composables/useAuth'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'

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
  <div class="min-h-screen flex items-center justify-center bg-gradient-to-br from-background via-background to-primary/5 px-4">
    <!-- Decorative background elements -->
    <div class="absolute inset-0 overflow-hidden pointer-events-none">
      <div class="absolute -top-40 -right-40 w-80 h-80 bg-primary/5 rounded-full blur-3xl" />
      <div class="absolute -bottom-40 -left-40 w-80 h-80 bg-primary/5 rounded-full blur-3xl" />
    </div>

    <Card class="w-full max-w-md relative">
      <CardHeader class="text-center pb-2">
        <div class="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-xl bg-primary text-primary-foreground">
          <CloudCog class="h-8 w-8" />
        </div>
        <CardTitle class="text-2xl font-bold">
          Welcome back
        </CardTitle>
        <CardDescription>Sign in to North Cloud Dashboard</CardDescription>
      </CardHeader>

      <CardContent>
        <form
          class="space-y-4"
          @submit.prevent="handleLogin"
        >
          <!-- Error message -->
          <div
            v-if="error"
            class="rounded-lg bg-destructive/10 p-4 text-sm text-destructive border border-destructive/20"
          >
            {{ error }}
          </div>

          <div class="space-y-2">
            <label
              for="username"
              class="text-sm font-medium"
            >Username</label>
            <Input
              id="username"
              v-model="username"
              type="text"
              placeholder="Enter your username"
              :disabled="loading"
              required
            />
          </div>

          <div class="space-y-2">
            <label
              for="password"
              class="text-sm font-medium"
            >Password</label>
            <Input
              id="password"
              v-model="password"
              type="password"
              placeholder="Enter your password"
              :disabled="loading"
              required
            />
          </div>

          <Button
            type="submit"
            class="w-full"
            :disabled="loading"
          >
            <Loader2
              v-if="loading"
              class="mr-2 h-4 w-4 animate-spin"
            />
            {{ loading ? 'Signing in...' : 'Sign in' }}
          </Button>
        </form>

        <div class="mt-6 text-center">
          <p class="text-xs text-muted-foreground">
            North Cloud Content Pipeline
          </p>
        </div>
      </CardContent>
    </Card>
  </div>
</template>
