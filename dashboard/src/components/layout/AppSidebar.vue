<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRoute } from 'vue-router'
import { ChevronLeft, ChevronDown, ChevronRight, Plus } from 'lucide-vue-next'
import { useSidebar } from '@/composables/useSidebar'
import { navigation, type NavSection } from '@/config/navigation'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Tooltip } from '@/components/ui/tooltip'
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
    isCollapsed.value ? 'w-16' : 'w-60'
  )
)
</script>

<template>
  <!-- Desktop Sidebar -->
  <aside
    v-if="!isMobile"
    :class="sidebarClass"
  >
    <!-- Logo/Brand -->
    <div class="flex h-14 items-center justify-between px-4 border-b border-sidebar-border">
      <router-link
        to="/"
        class="flex items-center gap-2.5"
      >
        <div class="h-7 w-7 rounded-sm bg-primary/10 border border-primary/30 flex items-center justify-center">
          <span class="text-primary font-mono font-bold text-xs">NC</span>
        </div>
        <span
          v-if="!isCollapsed"
          class="font-mono text-xs font-semibold tracking-[0.15em] uppercase text-sidebar-foreground"
        >North Cloud</span>
      </router-link>
      <Button
        v-if="!isCollapsed"
        variant="ghost"
        size="icon"
        class="h-7 w-7 text-sidebar-foreground/50 hover:text-sidebar-foreground"
        @click="toggle"
      >
        <ChevronLeft class="h-3.5 w-3.5" />
      </Button>
    </div>

    <!-- Navigation -->
    <nav class="flex-1 overflow-y-auto py-3 px-2">
      <ul class="space-y-0.5">
        <li
          v-for="section in navigation"
          :key="section.title"
          class="mb-1"
        >
          <!-- Section with no children (direct link) -->
          <template v-if="section.path && !section.children">
            <Tooltip
              v-if="isCollapsed"
              :content="section.title"
              side="right"
            >
              <router-link
                :to="section.path"
                :class="
                  cn(
                    'flex items-center gap-3 rounded-sm px-3 py-1.5 text-sidebar-foreground transition-colors relative',
                    isActive(section.path)
                      ? 'text-primary bg-primary/5 before:absolute before:left-0 before:top-1 before:bottom-1 before:w-0.5 before:bg-primary before:rounded-full'
                      : 'hover:bg-sidebar-accent/50 hover:text-sidebar-accent-foreground'
                  )
                "
              >
                <component
                  :is="section.icon"
                  class="h-4 w-4 shrink-0"
                />
                <span v-if="!isCollapsed">{{ section.title }}</span>
              </router-link>
            </Tooltip>
            <router-link
              v-else
              :to="section.path"
              :class="
                cn(
                  'flex items-center gap-3 rounded-sm px-3 py-1.5 text-sidebar-foreground transition-colors relative',
                  isActive(section.path)
                    ? 'text-primary bg-primary/5 before:absolute before:left-0 before:top-1 before:bottom-1 before:w-0.5 before:bg-primary before:rounded-full'
                    : 'hover:bg-sidebar-accent/50 hover:text-sidebar-accent-foreground'
                )
              "
            >
              <component
                :is="section.icon"
                class="h-4 w-4 shrink-0"
              />
              <span v-if="!isCollapsed">{{ section.title }}</span>
            </router-link>
          </template>

          <!-- Section with children (expandable) -->
          <template v-else-if="section.children">
            <!-- Collapsed view: show icon only with tooltip -->
            <template v-if="isCollapsed">
              <Tooltip
                :content="section.title"
                side="right"
              >
                <router-link
                  :to="section.children[0].path"
                  :class="
                    cn(
                      'flex items-center justify-center rounded-sm px-3 py-1.5 text-sidebar-foreground transition-colors relative',
                      isSectionActive(section)
                        ? 'text-primary bg-primary/5 before:absolute before:left-0 before:top-1 before:bottom-1 before:w-0.5 before:bg-primary before:rounded-full'
                        : 'hover:bg-sidebar-accent/50'
                    )
                  "
                >
                  <component
                    :is="section.icon"
                    class="h-4 w-4"
                  />
                </router-link>
              </Tooltip>
            </template>

            <!-- Expanded view: show section header and children -->
            <template v-else>
              <!-- Section label -->
              <button
                class="w-full flex items-center justify-between px-3 py-1.5 group"
                @click="toggleSection(section.title)"
              >
                <span class="text-[10px] font-mono font-medium uppercase tracking-[0.1em] text-sidebar-foreground/40">
                  {{ section.title }}
                </span>
                <ChevronDown
                  :class="
                    cn(
                      'h-3 w-3 text-sidebar-foreground/30 transition-transform group-hover:text-sidebar-foreground/50',
                      expandedSections.has(section.title) ? 'rotate-180' : ''
                    )
                  "
                />
              </button>

              <!-- Children -->
              <ul
                v-if="expandedSections.has(section.title)"
                class="space-y-0.5"
              >
                <li
                  v-for="child in section.children"
                  :key="child.path"
                >
                  <router-link
                    :to="child.path"
                    :class="
                      cn(
                        'flex items-center gap-2.5 rounded-sm px-3 py-1.5 text-sm text-sidebar-foreground transition-colors relative',
                        isActive(child.path)
                          ? 'text-primary bg-primary/5 before:absolute before:left-0 before:top-1 before:bottom-1 before:w-0.5 before:bg-primary before:rounded-full'
                          : 'hover:bg-sidebar-accent/50 hover:text-sidebar-accent-foreground'
                      )
                    "
                  >
                    <component
                      :is="child.icon"
                      class="h-3.5 w-3.5 shrink-0"
                    />
                    <span class="text-[13px]">{{ child.title }}</span>
                  </router-link>
                </li>

                <!-- Quick action -->
                <li v-if="section.quickAction">
                  <router-link
                    :to="section.quickAction.path"
                    class="flex items-center gap-2.5 rounded-sm px-3 py-1.5 text-sm text-primary/60 hover:text-primary hover:bg-primary/5 transition-colors"
                  >
                    <Plus class="h-3.5 w-3.5 shrink-0" />
                    <span class="text-[13px]">{{ section.quickAction.label }}</span>
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
      <Button
        variant="ghost"
        size="sm"
        class="w-full justify-center text-sidebar-foreground/40 hover:text-sidebar-foreground"
        @click="toggle"
      >
        <ChevronRight
          v-if="isCollapsed"
          class="h-3.5 w-3.5"
        />
        <template v-else>
          <ChevronLeft class="h-3.5 w-3.5 mr-2" />
          <span class="text-xs font-mono">Collapse</span>
        </template>
      </Button>
    </div>
  </aside>

  <!-- Mobile Sidebar (Sheet) -->
  <Sheet
    v-if="isMobile"
    :open="isMobileOpen"
    @update:open="closeMobile"
  >
    <SheetContent
      side="left"
      class="w-60 p-0 bg-sidebar"
    >
      <!-- Logo/Brand -->
      <div class="flex h-14 items-center px-4 border-b border-sidebar-border">
        <router-link
          to="/"
          class="flex items-center gap-2.5"
          @click="closeMobile"
        >
          <div class="h-7 w-7 rounded-sm bg-primary/10 border border-primary/30 flex items-center justify-center">
            <span class="text-primary font-mono font-bold text-xs">NC</span>
          </div>
          <span class="font-mono text-xs font-semibold tracking-[0.15em] uppercase text-sidebar-foreground">North Cloud</span>
        </router-link>
      </div>

      <!-- Navigation -->
      <nav class="flex-1 overflow-y-auto py-3 px-2">
        <ul class="space-y-0.5">
          <li
            v-for="section in navigation"
            :key="section.title"
            class="mb-1"
          >
            <!-- Section with no children -->
            <template v-if="section.path && !section.children">
              <router-link
                :to="section.path"
                :class="
                  cn(
                    'flex items-center gap-3 rounded-sm px-3 py-1.5 text-sidebar-foreground transition-colors relative',
                    isActive(section.path)
                      ? 'text-primary bg-primary/5 before:absolute before:left-0 before:top-1 before:bottom-1 before:w-0.5 before:bg-primary before:rounded-full'
                      : 'hover:bg-sidebar-accent/50'
                  )
                "
                @click="closeMobile"
              >
                <component
                  :is="section.icon"
                  class="h-4 w-4 shrink-0"
                />
                <span>{{ section.title }}</span>
              </router-link>
            </template>

            <!-- Section with children -->
            <template v-else-if="section.children">
              <button
                class="w-full flex items-center justify-between px-3 py-1.5 group"
                @click="toggleSection(section.title)"
              >
                <span class="text-[10px] font-mono font-medium uppercase tracking-[0.1em] text-sidebar-foreground/40">
                  {{ section.title }}
                </span>
                <ChevronDown
                  :class="
                    cn(
                      'h-3 w-3 text-sidebar-foreground/30 transition-transform group-hover:text-sidebar-foreground/50',
                      expandedSections.has(section.title) ? 'rotate-180' : ''
                    )
                  "
                />
              </button>

              <ul
                v-if="expandedSections.has(section.title)"
                class="space-y-0.5"
              >
                <li
                  v-for="child in section.children"
                  :key="child.path"
                >
                  <router-link
                    :to="child.path"
                    :class="
                      cn(
                        'flex items-center gap-2.5 rounded-sm px-3 py-1.5 text-sm text-sidebar-foreground transition-colors relative',
                        isActive(child.path)
                          ? 'text-primary bg-primary/5 before:absolute before:left-0 before:top-1 before:bottom-1 before:w-0.5 before:bg-primary before:rounded-full'
                          : 'hover:bg-sidebar-accent/50'
                      )
                    "
                    @click="closeMobile"
                  >
                    <component
                      :is="child.icon"
                      class="h-3.5 w-3.5 shrink-0"
                    />
                    <span class="text-[13px]">{{ child.title }}</span>
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
