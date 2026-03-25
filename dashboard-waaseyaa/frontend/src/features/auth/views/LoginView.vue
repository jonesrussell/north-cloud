<script setup lang="ts">
import { ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuth } from '@/shared/auth/useAuth'

const { login } = useAuth()
const router = useRouter()
const route = useRoute()

const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

async function handleLogin() {
  error.value = ''
  loading.value = true
  try {
    await login(username.value, password.value)
    const redirect = (route.query.redirect as string) || '/'
    router.push(redirect)
  } catch {
    error.value = 'Invalid credentials'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="min-h-screen bg-slate-950 flex items-center justify-center">
    <form @submit.prevent="handleLogin" class="w-80 space-y-4">
      <h1 class="text-2xl font-bold text-slate-100 text-center">North Cloud</h1>
      <div v-if="error" class="bg-red-900/50 text-red-300 p-3 rounded text-sm">{{ error }}</div>
      <input
        v-model="username"
        type="text"
        placeholder="Username"
        class="w-full px-3 py-2 bg-slate-800 border border-slate-700 rounded text-slate-100 placeholder-slate-500"
      />
      <input
        v-model="password"
        type="password"
        placeholder="Password"
        class="w-full px-3 py-2 bg-slate-800 border border-slate-700 rounded text-slate-100 placeholder-slate-500"
      />
      <button
        type="submit"
        :disabled="loading"
        class="w-full py-2 bg-blue-600 hover:bg-blue-500 text-white rounded font-medium disabled:opacity-50"
      >
        {{ loading ? 'Signing in...' : 'Sign In' }}
      </button>
    </form>
  </div>
</template>
