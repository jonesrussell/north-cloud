<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Menu, LogOut, User } from 'lucide-vue-next'
import { useSidebar } from '@/composables/useSidebar'
import { useAuth } from '@/composables/useAuth'
import { getBreadcrumbs } from '@/config/navigation'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb'
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuLabel,
} from '@/components/ui/dropdown-menu'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Badge } from '@/components/ui/badge'
import ThemeToggle from './ThemeToggle.vue'
import { ConnectionStatus } from '@/components/domain/realtime'
import { crawlerApi, publisherApi, classifierApi } from '@/api/client'
import { useUIStore } from '@/stores/ui'

const uiStore = useUIStore()
const isRealtimeEnabled = computed(() => uiStore.isFeatureEnabled('realtimeUpdates'))

const route = useRoute()
const router = useRouter()
const { isMobile, openMobile } = useSidebar()
const { logout } = useAuth()

// Health status
const healthStatus = ref<'healthy' | 'degraded' | 'unhealthy'>('healthy')

// Breadcrumbs
const breadcrumbs = computed(() => getBreadcrumbs(route.path))

// Page title from route meta
const pageTitle = computed(() => (route.meta?.title as string) || 'Dashboard')

// Handle logout
const handleLogout = () => {
  logout()
  router.push('/login')
}

// Check health on mount
onMounted(async () => {
  try {
    const results = await Promise.allSettled([
      crawlerApi.getHealth(),
      publisherApi.getHealth(),
      classifierApi.getHealth(),
    ])

    const healthy = results.filter((r) => r.status === 'fulfilled').length
    if (healthy === results.length) {
      healthStatus.value = 'healthy'
    } else if (healthy > 0) {
      healthStatus.value = 'degraded'
    } else {
      healthStatus.value = 'unhealthy'
    }
  } catch {
    healthStatus.value = 'unhealthy'
  }
})

const healthBadgeVariant = computed(() => {
  switch (healthStatus.value) {
    case 'healthy':
      return 'success'
    case 'degraded':
      return 'warning'
    case 'unhealthy':
    default:
      return 'destructive'
  }
})

const healthLabel = computed(() => {
  switch (healthStatus.value) {
    case 'healthy':
      return 'OK'
    case 'degraded':
      return 'DEGRADED'
    case 'unhealthy':
    default:
      return 'DOWN'
  }
})
</script>

<template>
  <header
    :class="cn(
      'sticky top-0 z-40 flex h-14 items-center gap-4 border-b bg-background/95 backdrop-blur-sm px-4 md:px-6',
      healthStatus === 'healthy' ? 'border-b-primary/10' : ''
    )"
  >
    <!-- Mobile menu button -->
    <Button
      v-if="isMobile"
      variant="ghost"
      size="icon"
      class="h-8 w-8"
      @click="openMobile"
    >
      <Menu class="h-4 w-4" />
      <span class="sr-only">Toggle menu</span>
    </Button>

    <!-- Breadcrumbs -->
    <Breadcrumb class="hidden md:flex">
      <template
        v-for="(crumb, index) in breadcrumbs"
        :key="`${index}-${crumb.path}`"
      >
        <BreadcrumbItem>
          <BreadcrumbLink
            v-if="index < breadcrumbs.length - 1"
            :href="crumb.path"
          >
            <router-link
              :to="crumb.path"
              class="font-mono text-xs text-muted-foreground hover:text-foreground transition-colors"
            >
              {{ crumb.label }}
            </router-link>
          </BreadcrumbLink>
          <BreadcrumbPage
            v-else
            class="font-mono text-xs"
          >
            {{ crumb.label }}
          </BreadcrumbPage>
        </BreadcrumbItem>
        <BreadcrumbSeparator v-if="index < breadcrumbs.length - 1">
          <span class="font-mono text-muted-foreground/50">/</span>
        </BreadcrumbSeparator>
      </template>
    </Breadcrumb>

    <!-- Page title (mobile) -->
    <h1 class="md:hidden font-mono text-sm font-medium tracking-tight">
      {{ pageTitle }}
    </h1>

    <!-- Spacer -->
    <div class="flex-1" />

    <!-- Right side actions -->
    <div class="flex items-center gap-2">
      <!-- Real-time Connection Status -->
      <ConnectionStatus
        v-if="isRealtimeEnabled"
        :show-label="false"
        class="hidden sm:flex"
      />

      <!-- Health Status -->
      <Badge
        :variant="healthBadgeVariant"
        class="hidden sm:flex font-mono text-[10px] tracking-wider"
      >
        <span
          :class="
            cn('mr-1.5 h-1.5 w-1.5 rounded-full', {
              'bg-green-500 animate-pulse-glow': healthStatus === 'healthy',
              'bg-amber-500': healthStatus === 'degraded',
              'bg-red-500': healthStatus === 'unhealthy',
            })
          "
        />
        {{ healthLabel }}
      </Badge>

      <!-- Theme Toggle -->
      <ThemeToggle />

      <!-- User Menu -->
      <DropdownMenu>
        <DropdownMenuTrigger>
          <Button
            variant="ghost"
            class="relative h-8 w-8 rounded-sm"
          >
            <Avatar class="h-8 w-8 rounded-sm">
              <AvatarFallback class="rounded-sm bg-primary/10 text-primary">
                <User class="h-3.5 w-3.5" />
              </AvatarFallback>
            </Avatar>
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent
          align="end"
          class="w-48"
        >
          <DropdownMenuLabel class="font-mono text-xs">
            Account
          </DropdownMenuLabel>
          <DropdownMenuSeparator />
          <DropdownMenuItem @select="handleLogout">
            <LogOut class="mr-2 h-3.5 w-3.5" />
            <span class="text-sm">Log out</span>
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  </header>
</template>
