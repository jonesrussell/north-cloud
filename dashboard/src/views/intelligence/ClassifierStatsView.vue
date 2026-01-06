<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Loader2, BarChart3, TrendingUp, FileText, Target } from 'lucide-vue-next'
import { classifierApi } from '@/api/client'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { MetricCard } from '@/components/pipeline'

interface ClassifierStats {
  total_classified: number
  avg_quality_score: number
  crime_articles: number
  topics_count: number
  today_classified: number
  weekly_change: number
}

const loading = ref(true)
const error = ref<string | null>(null)
const stats = ref<ClassifierStats | null>(null)
const topTopics = ref<Array<{ name: string; count: number }>>([])

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
      <h1 class="text-3xl font-bold tracking-tight">Classifier Statistics</h1>
      <p class="text-muted-foreground">Content classification performance and insights</p>
    </div>

    <div v-if="loading" class="flex items-center justify-center py-12">
      <Loader2 class="h-8 w-8 animate-spin text-muted-foreground" />
    </div>

    <Card v-else-if="error" class="border-destructive">
      <CardContent class="pt-6">
        <p class="text-destructive">{{ error }}</p>
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
          :value="`${stats?.avg_quality_score || 0}/100`"
          subtitle="content quality"
          :icon="Target"
        />
        <MetricCard
          title="Crime Articles"
          :value="stats?.crime_articles?.toLocaleString() || '0'"
          subtitle="flagged as crime"
          :icon="BarChart3"
        />
        <MetricCard
          title="Today"
          :value="stats?.today_classified?.toLocaleString() || '0'"
          subtitle="classified today"
          :change="stats?.weekly_change"
          :trend="(stats?.weekly_change || 0) > 0 ? 'up' : 'down'"
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
          <div v-if="topTopics.length === 0" class="text-center py-8 text-muted-foreground">
            No topics data available yet.
          </div>
          <div v-else class="space-y-4">
            <div v-for="topic in topTopics.slice(0, 10)" :key="topic.name" class="flex items-center">
              <div class="flex-1">
                <div class="flex items-center justify-between mb-1">
                  <span class="text-sm font-medium">{{ topic.name }}</span>
                  <span class="text-sm text-muted-foreground">{{ topic.count.toLocaleString() }}</span>
                </div>
                <div class="h-2 bg-muted rounded-full overflow-hidden">
                  <div
                    class="h-full bg-primary rounded-full"
                    :style="{ width: `${(topic.count / (topTopics[0]?.count || 1)) * 100}%` }"
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
