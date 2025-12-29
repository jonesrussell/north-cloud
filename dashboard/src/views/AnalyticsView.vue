<template>
  <div>
    <!-- Page Header -->
    <PageHeader
      title="System Analytics"
      subtitle="Consolidated statistics and metrics across all services"
    />

    <!-- Tab Navigation -->
    <div class="mt-6">
      <div class="border-b border-gray-200">
        <nav
          class="-mb-px flex space-x-8"
          aria-label="Tabs"
        >
          <button
            v-for="tab in tabs"
            :key="tab.id"
            :class="[
              activeTab === tab.id
                ? 'border-blue-500 text-blue-600'
                : 'border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-700',
              'group inline-flex items-center border-b-2 py-4 px-1 text-sm font-medium transition-colors',
            ]"
            @click="activeTab = tab.id"
          >
            <component
              :is="tab.icon"
              :class="[
                activeTab === tab.id ? 'text-blue-500' : 'text-gray-400 group-hover:text-gray-500',
                '-ml-0.5 mr-2 h-5 w-5',
              ]"
            />
            {{ tab.label }}
          </button>
        </nav>
      </div>

      <!-- Tab Panels -->
      <div class="mt-6">
        <!-- Crawler Stats -->
        <div
          v-show="activeTab === 'crawler'"
          class="animate-fade-in"
        >
          <CrawlerStatsView />
        </div>

        <!-- Classifier Stats -->
        <div
          v-show="activeTab === 'classifier'"
          class="animate-fade-in"
        >
          <ClassifierStatsView />
        </div>

        <!-- Publisher Stats -->
        <div
          v-show="activeTab === 'publisher'"
          class="animate-fade-in"
        >
          <PublisherStatsView />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  FunnelIcon,
  MegaphoneIcon,
  ChartBarIcon,
} from '@heroicons/vue/24/outline'
import { PageHeader } from '@/components/common'

// Import stats view components
import CrawlerStatsView from './crawler/StatsView.vue'
import ClassifierStatsView from './classifier/StatsView.vue'
import PublisherStatsView from './publisher/StatsView.vue'

const route = useRoute()
const router = useRouter()

// Tab configuration
const tabs = [
  {
    id: 'crawler',
    label: 'Crawler',
    icon: FunnelIcon,
  },
  {
    id: 'classifier',
    label: 'Classifier',
    icon: ChartBarIcon,
  },
  {
    id: 'publisher',
    label: 'Publisher',
    icon: MegaphoneIcon,
  },
]

// Active tab state (sync with query param)
const activeTab = ref<string>('crawler')

// Initialize active tab from query param
onMounted(() => {
  const tabParam = route.query.tab as string
  if (tabParam && tabs.some((t) => t.id === tabParam)) {
    activeTab.value = tabParam
  }
})

// Watch for tab changes and update URL
watch(activeTab, (newTab) => {
  // Update query param without navigating
  router.replace({
    query: { ...route.query, tab: newTab },
  })
})

// Watch for query param changes (browser back/forward)
watch(
  () => route.query.tab,
  (newTab) => {
    if (newTab && tabs.some((t) => t.id === newTab)) {
      activeTab.value = newTab as string
    }
  }
)
</script>

<style scoped>
.animate-fade-in {
  animation: fadeIn 0.2s ease-in;
}

@keyframes fadeIn {
  from {
    opacity: 0;
  }
  to {
    opacity: 1;
  }
}
</style>
