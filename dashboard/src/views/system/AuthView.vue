<script setup lang="ts">
import { ref } from 'vue'
import { Shield, Key, Clock, User } from 'lucide-vue-next'
import { useAuth } from '@/composables/useAuth'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'

const { isAuthenticated, logout } = useAuth()

// Get token info from localStorage
const tokenInfo = ref({
  exists: !!localStorage.getItem('dashboard_token'),
  createdAt: localStorage.getItem('dashboard_token_created') || 'Unknown',
})

const handleLogout = () => {
  logout()
}
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-3xl font-bold tracking-tight">Authentication</h1>
      <p class="text-muted-foreground">Manage your session and security settings</p>
    </div>

    <!-- Session Status -->
    <Card>
      <CardHeader>
        <CardTitle class="flex items-center gap-2">
          <Shield class="h-5 w-5" />
          Session Status
        </CardTitle>
        <CardDescription>Your current authentication state</CardDescription>
      </CardHeader>
      <CardContent>
        <div class="space-y-4">
          <div class="flex items-center justify-between p-4 bg-muted rounded-lg">
            <div class="flex items-center gap-3">
              <User class="h-5 w-5 text-muted-foreground" />
              <div>
                <p class="font-medium">Authenticated</p>
                <p class="text-sm text-muted-foreground">You are logged in</p>
              </div>
            </div>
            <div 
              :class="[
                'h-3 w-3 rounded-full',
                isAuthenticated ? 'bg-green-500' : 'bg-red-500'
              ]"
            />
          </div>

          <div class="flex items-center justify-between p-4 bg-muted rounded-lg">
            <div class="flex items-center gap-3">
              <Key class="h-5 w-5 text-muted-foreground" />
              <div>
                <p class="font-medium">JWT Token</p>
                <p class="text-sm text-muted-foreground">{{ tokenInfo.exists ? 'Valid token stored' : 'No token found' }}</p>
              </div>
            </div>
          </div>

          <div class="flex items-center justify-between p-4 bg-muted rounded-lg">
            <div class="flex items-center gap-3">
              <Clock class="h-5 w-5 text-muted-foreground" />
              <div>
                <p class="font-medium">Token Expiry</p>
                <p class="text-sm text-muted-foreground">Tokens expire after 24 hours</p>
              </div>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>

    <!-- Actions -->
    <Card>
      <CardHeader>
        <CardTitle>Session Actions</CardTitle>
        <CardDescription>Manage your current session</CardDescription>
      </CardHeader>
      <CardContent>
        <div class="flex gap-4">
          <Button variant="destructive" @click="handleLogout">
            Sign Out
          </Button>
        </div>
        <p class="mt-4 text-sm text-muted-foreground">
          Signing out will clear your authentication token and redirect you to the login page.
        </p>
      </CardContent>
    </Card>
  </div>
</template>
