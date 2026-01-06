<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRoute } from 'vue-router'
import { ChevronLeft, ChevronDown, ChevronRight, Plus } from 'lucide-vue-next'
import { useSidebar } from '@/composables/useSidebar'
import { navigation, type NavSection } from '@/config/navigation'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Tooltip } from '@/components/ui/tooltip'
import { Separator } from '@/components/ui/separator'
import { Sheet, SheetContent } from '@/components/ui/sheet'

const route = useRoute()
const { isCollapsed, isMobile, isMobileOpen, toggle, closeMobile } = useSidebar()

// Track expanded sections
const expandedSections = ref<Set<string>>(new Set())

const toggleSection = (title: string) => {
  if (expandedSections.value.has(title)) {
    expandedSections.value.delete(title)
  } else {
    expandedSections.value.add(title)
  }
}

const isActive = (path: string): boolean => {
  return route.path === path || route.path.startsWith(path + '/')
}

const isSectionActive = (section: NavSection): boolean => {
  if (section.path && isActive(section.path)) return true
  if (section.children) {
    return section.children.some((child) => isActive(child.path))
  }
  return false
}

// Auto-expand active section
const autoExpandActive = () => {
  for (const section of navigation) {
    if (section.children && isSectionActive(section)) {
      expandedSections.value.add(section.title)
    }
  }
}
autoExpandActive()

const sidebarClass = computed(() =>
  cn(
    'fixed inset-y-0 left-0 z-50 flex flex-col bg-sidebar border-r border-sidebar-border transition-all duration-200 ease-in-out',
    isCollapsed.value ? 'w-16' : 'w-64'
  )
)
</script>

<template>
  <!-- Desktop Sidebar -->
  <aside v-if="!isMobile" :class="sidebarClass">
    <!-- Logo/Brand -->
    <div class="flex h-16 items-center justify-between px-4 border-b border-sidebar-border">
      <router-link to="/" class="flex items-center gap-2">
        <div class="h-8 w-8 rounded-lg bg-sidebar-primary flex items-center justify-center">
          <span class="text-sidebar-primary-foreground font-bold text-sm">NC</span>
        </div>
        <span v-if="!isCollapsed" class="font-semibold text-sidebar-foreground">North Cloud</span>
      </router-link>
      <Button v-if="!isCollapsed" variant="ghost" size="icon" class="h-8 w-8" @click="toggle">
        <ChevronLeft class="h-4 w-4" />
      </Button>
    </div>

    <!-- Navigation -->
    <nav class="flex-1 overflow-y-auto py-4 px-2">
      <ul class="space-y-1">
        <li v-for="section in navigation" :key="section.title">
          <!-- Section with no children (direct link) -->
          <template v-if="section.path && !section.children">
            <Tooltip v-if="isCollapsed" :content="section.title" side="right">
              <router-link
                :to="section.path"
                :class="
                  cn(
                    'flex items-center gap-3 rounded-lg px-3 py-2 text-sidebar-foreground transition-colors',
                    isActive(section.path)
                      ? 'bg-sidebar-accent text-sidebar-accent-foreground'
                      : 'hover:bg-sidebar-accent/50'
                  )
                "
              >
                <component :is="section.icon" class="h-5 w-5 shrink-0" />
                <span v-if="!isCollapsed">{{ section.title }}</span>
              </router-link>
            </Tooltip>
            <router-link
              v-else
              :to="section.path"
              :class="
                cn(
                  'flex items-center gap-3 rounded-lg px-3 py-2 text-sidebar-foreground transition-colors',
                  isActive(section.path)
                    ? 'bg-sidebar-accent text-sidebar-accent-foreground'
                    : 'hover:bg-sidebar-accent/50'
                )
              "
            >
              <component :is="section.icon" class="h-5 w-5 shrink-0" />
              <span v-if="!isCollapsed">{{ section.title }}</span>
            </router-link>
          </template>

          <!-- Section with children (expandable) -->
          <template v-else-if="section.children">
            <!-- Collapsed view: show icon only with tooltip -->
            <template v-if="isCollapsed">
              <Tooltip :content="section.title" side="right">
                <router-link
                  :to="section.children[0].path"
                  :class="
                    cn(
                      'flex items-center justify-center rounded-lg px-3 py-2 text-sidebar-foreground transition-colors',
                      isSectionActive(section)
                        ? 'bg-sidebar-accent text-sidebar-accent-foreground'
                        : 'hover:bg-sidebar-accent/50'
                    )
                  "
                >
                  <component :is="section.icon" class="h-5 w-5" />
                </router-link>
              </Tooltip>
            </template>

            <!-- Expanded view: show section header and children -->
            <template v-else>
              <!-- Section header -->
              <button
                @click="toggleSection(section.title)"
                :class="
                  cn(
                    'w-full flex items-center justify-between gap-3 rounded-lg px-3 py-2 text-sidebar-foreground transition-colors',
                    isSectionActive(section)
                      ? 'bg-sidebar-accent/50 text-sidebar-accent-foreground'
                      : 'hover:bg-sidebar-accent/50'
                  )
                "
              >
                <div class="flex items-center gap-3">
                  <component :is="section.icon" class="h-5 w-5 shrink-0" />
                  <span class="text-sm font-medium">{{ section.title }}</span>
                </div>
                <ChevronDown
                  :class="
                    cn(
                      'h-4 w-4 transition-transform',
                      expandedSections.has(section.title) ? 'rotate-180' : ''
                    )
                  "
                />
              </button>

              <!-- Children -->
              <ul
                v-if="expandedSections.has(section.title)"
                class="mt-1 ml-4 pl-4 border-l border-sidebar-border space-y-1"
              >
                <li v-for="child in section.children" :key="child.path">
                  <router-link
                    :to="child.path"
                    :class="
                      cn(
                        'flex items-center gap-3 rounded-lg px-3 py-2 text-sm text-sidebar-foreground transition-colors',
                        isActive(child.path)
                          ? 'bg-sidebar-accent text-sidebar-accent-foreground'
                          : 'hover:bg-sidebar-accent/50'
                      )
                    "
                  >
                    <component :is="child.icon" class="h-4 w-4 shrink-0" />
                    <span>{{ child.title }}</span>
                  </router-link>
                </li>

                <!-- Quick action -->
                <li v-if="section.quickAction">
                  <router-link
                    :to="section.quickAction.path"
                    class="flex items-center gap-3 rounded-lg px-3 py-2 text-sm text-sidebar-primary hover:bg-sidebar-accent/50 transition-colors"
                  >
                    <Plus class="h-4 w-4 shrink-0" />
                    <span>{{ section.quickAction.label }}</span>
                  </router-link>
                </li>
              </ul>
            </template>
          </template>
        </li>
      </ul>
    </nav>

    <!-- Collapse toggle (bottom) -->
    <div class="border-t border-sidebar-border p-2">
      <Button variant="ghost" size="sm" class="w-full justify-center" @click="toggle">
        <ChevronRight v-if="isCollapsed" class="h-4 w-4" />
        <template v-else>
          <ChevronLeft class="h-4 w-4 mr-2" />
          <span>Collapse</span>
        </template>
      </Button>
    </div>
  </aside>

  <!-- Mobile Sidebar (Sheet) -->
  <Sheet v-if="isMobile" :open="isMobileOpen" @update:open="closeMobile">
    <SheetContent side="left" class="w-64 p-0">
      <!-- Logo/Brand -->
      <div class="flex h-16 items-center px-4 border-b border-sidebar-border">
        <router-link to="/" class="flex items-center gap-2" @click="closeMobile">
          <div class="h-8 w-8 rounded-lg bg-sidebar-primary flex items-center justify-center">
            <span class="text-sidebar-primary-foreground font-bold text-sm">NC</span>
          </div>
          <span class="font-semibold text-sidebar-foreground">North Cloud</span>
        </router-link>
      </div>

      <!-- Navigation -->
      <nav class="flex-1 overflow-y-auto py-4 px-2">
        <ul class="space-y-1">
          <li v-for="section in navigation" :key="section.title">
            <!-- Section with no children -->
            <template v-if="section.path && !section.children">
              <router-link
                :to="section.path"
                @click="closeMobile"
                :class="
                  cn(
                    'flex items-center gap-3 rounded-lg px-3 py-2 text-sidebar-foreground transition-colors',
                    isActive(section.path)
                      ? 'bg-sidebar-accent text-sidebar-accent-foreground'
                      : 'hover:bg-sidebar-accent/50'
                  )
                "
              >
                <component :is="section.icon" class="h-5 w-5 shrink-0" />
                <span>{{ section.title }}</span>
              </router-link>
            </template>

            <!-- Section with children -->
            <template v-else-if="section.children">
              <button
                @click="toggleSection(section.title)"
                :class="
                  cn(
                    'w-full flex items-center justify-between gap-3 rounded-lg px-3 py-2 text-sidebar-foreground transition-colors',
                    isSectionActive(section)
                      ? 'bg-sidebar-accent/50 text-sidebar-accent-foreground'
                      : 'hover:bg-sidebar-accent/50'
                  )
                "
              >
                <div class="flex items-center gap-3">
                  <component :is="section.icon" class="h-5 w-5 shrink-0" />
                  <span class="text-sm font-medium">{{ section.title }}</span>
                </div>
                <ChevronDown
                  :class="
                    cn(
                      'h-4 w-4 transition-transform',
                      expandedSections.has(section.title) ? 'rotate-180' : ''
                    )
                  "
                />
              </button>

              <ul
                v-if="expandedSections.has(section.title)"
                class="mt-1 ml-4 pl-4 border-l border-sidebar-border space-y-1"
              >
                <li v-for="child in section.children" :key="child.path">
                  <router-link
                    :to="child.path"
                    @click="closeMobile"
                    :class="
                      cn(
                        'flex items-center gap-3 rounded-lg px-3 py-2 text-sm text-sidebar-foreground transition-colors',
                        isActive(child.path)
                          ? 'bg-sidebar-accent text-sidebar-accent-foreground'
                          : 'hover:bg-sidebar-accent/50'
                      )
                    "
                  >
                    <component :is="child.icon" class="h-4 w-4 shrink-0" />
                    <span>{{ child.title }}</span>
                  </router-link>
                </li>
              </ul>
            </template>
          </li>
        </ul>
      </nav>
    </SheetContent>
  </Sheet>
</template>
