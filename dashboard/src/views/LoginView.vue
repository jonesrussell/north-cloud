<template>
  <div class="flex min-h-screen items-center justify-center bg-gray-100 px-4 py-12 sm:px-6 lg:px-8">
    <div class="w-full max-w-md space-y-8">
      <div>
        <div class="mx-auto flex h-16 w-16 items-center justify-center rounded-full bg-blue-600">
          <svg
            class="h-8 w-8 text-white"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"
            />
          </svg>
        </div>
        <h2 class="mt-6 text-center text-3xl font-extrabold text-gray-900">
          Sign in to North Cloud
        </h2>
        <p class="mt-2 text-center text-sm text-gray-600">
          Dashboard access
        </p>
      </div>

      <form
        class="mt-8 space-y-6"
        @submit.prevent="handleLogin"
      >
        <div
          v-if="error"
          class="rounded-md bg-red-50 p-4"
        >
          <div class="flex">
            <div class="flex-shrink-0">
              <svg
                class="h-5 w-5 text-red-400"
                viewBox="0 0 20 20"
                fill="currentColor"
              >
                <path
                  fill-rule="evenodd"
                  d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z"
                  clip-rule="evenodd"
                />
              </svg>
            </div>
            <div class="ml-3">
              <p class="text-sm font-medium text-red-800">
                {{ error }}
              </p>
            </div>
          </div>
        </div>

        <div class="space-y-4 rounded-md shadow-sm">
          <div>
            <label
              for="username"
              class="block text-sm font-medium text-gray-700"
            >
              Username
            </label>
            <input
              id="username"
              v-model="username"
              name="username"
              type="text"
              required
              class="mt-1 block w-full appearance-none rounded-md border border-gray-300 px-3 py-2 placeholder-gray-400 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500 sm:text-sm"
              placeholder="Enter username"
              :disabled="loading"
            >
          </div>

          <div>
            <label
              for="password"
              class="block text-sm font-medium text-gray-700"
            >
              Password
            </label>
            <input
              id="password"
              v-model="password"
              name="password"
              type="password"
              required
              class="mt-1 block w-full appearance-none rounded-md border border-gray-300 px-3 py-2 placeholder-gray-400 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500 sm:text-sm"
              placeholder="Enter password"
              :disabled="loading"
            >
          </div>
        </div>

        <div>
          <button
            type="submit"
            :disabled="loading"
            class="group relative flex w-full justify-center rounded-md border border-transparent bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <span v-if="loading">Signing in...</span>
            <span v-else>Sign in</span>
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuth } from '../composables/useAuth'

const router = useRouter()
const { login, isAuthenticated } = useAuth()

const username = ref('')
const password = ref('')
const loading = ref(false)
const error = ref(null)

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
      // Redirect to dashboard home
      router.push('/')
    } else {
      error.value = result.error || 'Login failed'
    }
  } catch (err) {
    error.value = err.message || 'An unexpected error occurred'
  } finally {
    loading.value = false
  }
}
</script>

