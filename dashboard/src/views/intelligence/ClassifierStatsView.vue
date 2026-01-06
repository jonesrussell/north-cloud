<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Loader2, BarChart3, TrendingUp, FileText, Target } from 'lucide-vue-next'
import { classifierApi } from '@/api/client'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { MetricCard } from '@/components/pipeline'

// Match the actual API response from classifier service
interface ClassifierStats {
  total_classified: number
  avg_quality_score: number
  crime_related: number
  avg_processing_time_ms: number
  content_types: Record<string, number>
}

interface TopicStat {
  topic: string
  count: number
  avg_quality?: number
}

const loading = ref(true)
const error = ref<string | null>(null)
const stats = ref<ClassifierStats | null>(null)
const topTopics = ref<TopicStat[]>([])

const loadStats = async () => {
  try {
    loading.value = true
    const [statsRes, topicsRes] = await Promise.all([
      classifierApi.stats.get(),
      classifierApi.stats.topics(),
    ])
    stats.value = statsRes.data
    topTopics.value = topicsRes.data?.topics || []
  } catch (err) {
    error.value = 'Unable to load classifier statistics.'
  } finally {
    loading.value = false
  }
}

onMounted(loadStats)
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-3xl font-bold tracking-tight">
        Classifier Statistics
      </h1>
      <p class="text-muted-foreground">
        Content classification performance and insights
      </p>
    </div>

    <div
      v-if="loading"
      class="flex items-center justify-center py-12"
    >
      <Loader2 class="h-8 w-8 animate-spin text-muted-foreground" />
    </div>

    <Card
      v-else-if="error"
      class="border-destructive"
    >
      <CardContent class="pt-6">
        <p class="text-destructive">
          {{ error }}
        </p>
      </CardContent>
    </Card>

    <template v-else>
      <!-- Key Metrics -->
      <div class="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <MetricCard
          title="Total Classified"
          :value="stats?.total_classified?.toLocaleString() || '0'"
          subtitle="articles processed"
          :icon="FileText"
        />
        <MetricCard
          title="Avg Quality Score"
          :value="`${(stats?.avg_quality_score || 0).toFixed(1)}/100`"
          subtitle="content quality"
          :icon="Target"
        />
        <MetricCard
          title="Crime Related"
          :value="stats?.crime_related?.toLocaleString() || '0'"
          subtitle="flagged as crime"
          :icon="BarChart3"
        />
        <MetricCard
          title="Avg Processing"
          :value="`${(stats?.avg_processing_time_ms || 0).toFixed(0)}ms`"
          subtitle="per article"
          :icon="TrendingUp"
        />
      </div>

      <!-- Top Topics -->
      <Card>
        <CardHeader>
          <CardTitle>Top Topics</CardTitle>
          <CardDescription>Most common topics detected in classified content</CardDescription>
        </CardHeader>
        <CardContent>
          <div
            v-if="topTopics.length === 0"
            class="text-center py-8 text-muted-foreground"
          >
            No topics data available yet.
          </div>
          <div
            v-else
            class="space-y-4"
          >
            <div
              v-for="item in topTopics.slice(0, 10)"
              :key="item.topic"
              class="flex items-center"
            >
              <div class="flex-1">
                <div class="flex items-center justify-between mb-1">
                  <span class="text-sm font-medium">{{ item.topic }}</span>
                  <span class="text-sm text-muted-foreground">{{ item.count.toLocaleString() }}</span>
                </div>
                <div class="h-2 bg-muted rounded-full overflow-hidden">
                  <div
                    class="h-full bg-primary rounded-full"
                    :style="{ width: `${(item.count / (topTopics[0]?.count || 1)) * 100}%` }"
                  />
                </div>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>
    </template>
  </div>
</template>
