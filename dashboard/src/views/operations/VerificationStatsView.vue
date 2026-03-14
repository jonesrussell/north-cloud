<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowLeft, RefreshCw } from 'lucide-vue-next'
import { verificationApi, type VerificationStats } from '@/api/verification'
import { Button } from '@/components/ui/button'
import { StatCard, LoadingSpinner, ErrorAlert } from '@/components/common'

const router = useRouter()

const loading = ref(true)
const error = ref<string | null>(null)
const stats = ref<VerificationStats | null>(null)

async function loadStats() {
  loading.value = true
  error.value = null
  try {
    stats.value = await verificationApi.getStats()
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : 'Failed to load stats'
  } finally {
    loading.value = false
  }
}

onMounted(loadStats)
</script>

<template>
  <div class="p-6 space-y-6">
    <div class="flex items-center justify-between">
      <div class="flex items-center gap-3">
        <Button
          variant="ghost"
          size="sm"
          @click="router.push({ name: 'operations-verification' })"
        >
          <ArrowLeft class="h-4 w-4 mr-1" />
          Back to Queue
        </Button>
        <h1 class="text-xl font-semibold">
          Verification Stats
        </h1>
      </div>
      <Button
        variant="outline"
        size="sm"
        @click="loadStats"
      >
        <RefreshCw class="h-4 w-4 mr-1" />
        Refresh
      </Button>
    </div>

    <LoadingSpinner v-if="loading" />
    <ErrorAlert
      v-else-if="error"
      :message="error"
    />

    <template v-else-if="stats">
      <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
        <StatCard
          title="Pending People"
          :value="stats.pending_people"
          description="Awaiting review"
        />
        <StatCard
          title="Pending Band Offices"
          :value="stats.pending_band_offices"
          description="Awaiting review"
        />
        <StatCard
          title="AI-Scored People"
          :value="stats.scored_people"
          description="Of pending"
        />
        <StatCard
          title="AI-Scored Band Offices"
          :value="stats.scored_band_offices"
          description="Of pending"
        />
      </div>

      <div>
        <h2 class="text-lg font-medium mb-3">
          Confidence Distribution
        </h2>
        <div class="grid grid-cols-3 gap-4">
          <StatCard
            title="High Confidence"
            :value="stats.high_confidence"
            description="≥ 90% — safe to bulk verify"
            class="border-green-200"
          />
          <StatCard
            title="Medium Confidence"
            :value="stats.medium_confidence"
            description="50–89% — review recommended"
            class="border-yellow-200"
          />
          <StatCard
            title="Low Confidence"
            :value="stats.low_confidence"
            description="< 50% — likely reject"
            class="border-red-200"
          />
        </div>
      </div>
    </template>
  </div>
</template>
