<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Loader2, FileText, ExternalLink, RefreshCw } from 'lucide-vue-next'
import { publisherApi } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'

interface Article {
  title: string
  url: string
  quality_score: number
  topics: string[]
  channel: string
  published_at: string
}

const loading = ref(true)
const error = ref<string | null>(null)
const articles = ref<Article[]>([])

const loadArticles = async () => {
  try {
    loading.value = true
    const response = await publisherApi.articles.recent({ limit: 50 })
    articles.value = response.data?.articles || []
  } catch (err) {
    error.value = 'Unable to load recent articles.'
  } finally {
    loading.value = false
  }
}

const formatDate = (date: string) => {
  if (!date) return 'N/A'
  const d = new Date(date)
  return d.toLocaleString()
}

onMounted(loadArticles)
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-3xl font-bold tracking-tight">Recent Articles</h1>
        <p class="text-muted-foreground">Recently published articles across all channels</p>
      </div>
      <Button variant="outline" @click="loadArticles">
        <RefreshCw class="mr-2 h-4 w-4" />
        Refresh
      </Button>
    </div>

    <div v-if="loading" class="flex items-center justify-center py-12">
      <Loader2 class="h-8 w-8 animate-spin text-muted-foreground" />
    </div>

    <Card v-else-if="error" class="border-destructive">
      <CardContent class="pt-6">
        <p class="text-destructive">{{ error }}</p>
      </CardContent>
    </Card>

    <Card v-else-if="articles.length === 0">
      <CardContent class="flex flex-col items-center justify-center py-12">
        <FileText class="h-12 w-12 text-muted-foreground mb-4" />
        <h3 class="text-lg font-medium mb-2">No recent articles</h3>
        <p class="text-muted-foreground">Articles will appear here once published to channels.</p>
      </CardContent>
    </Card>

    <Card v-else>
      <CardHeader>
        <CardTitle>Published Articles</CardTitle>
        <CardDescription>Showing the {{ articles.length }} most recent articles</CardDescription>
      </CardHeader>
      <CardContent class="p-0">
        <table class="w-full">
          <thead class="border-b bg-muted/50">
            <tr>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">Title</th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">Channel</th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">Quality</th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">Published</th>
              <th class="px-6 py-3 text-right text-xs font-medium text-muted-foreground uppercase">Link</th>
            </tr>
          </thead>
          <tbody class="divide-y">
            <tr v-for="(article, index) in articles" :key="index" class="hover:bg-muted/50">
              <td class="px-6 py-4">
                <p class="text-sm font-medium truncate max-w-sm">{{ article.title }}</p>
                <div class="flex gap-1 mt-1">
                  <Badge v-for="topic in article.topics?.slice(0, 2)" :key="topic" variant="outline" class="text-xs">
                    {{ topic }}
                  </Badge>
                </div>
              </td>
              <td class="px-6 py-4">
                <Badge variant="secondary">{{ article.channel }}</Badge>
              </td>
              <td class="px-6 py-4 text-sm text-muted-foreground">
                {{ article.quality_score }}/100
              </td>
              <td class="px-6 py-4 text-sm text-muted-foreground">
                {{ formatDate(article.published_at) }}
              </td>
              <td class="px-6 py-4 text-right">
                <a :href="article.url" target="_blank" class="text-primary hover:text-primary/80">
                  <ExternalLink class="h-4 w-4" />
                </a>
              </td>
            </tr>
          </tbody>
        </table>
      </CardContent>
    </Card>
  </div>
</template>
