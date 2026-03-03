<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useQuery } from '@tanstack/vue-query'
import { toast } from 'vue-sonner'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import PublishForm from '@/components/domain/social-publishing/PublishForm.vue'
import { socialPublisherApi } from '@/api/client'
import type { SocialAccount, PublishRequest } from '@/types/socialPublisher'

const router = useRouter()
const publishing = ref(false)

const { data: accountsData, isLoading: accountsLoading } = useQuery({
  queryKey: ['social-publisher', 'accounts', 'list'],
  queryFn: async (): Promise<SocialAccount[]> => {
    const response = await socialPublisherApi.accounts.list()
    return response.data?.items ?? []
  },
})

async function handlePublish(data: PublishRequest) {
  publishing.value = true
  try {
    await socialPublisherApi.content.publish(data)
    toast.success(data.scheduled_at ? 'Content scheduled' : 'Content published')
    router.push('/distribution/social-content')
  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : 'Failed to publish content'
    toast.error(message)
  } finally {
    publishing.value = false
  }
}
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-3xl font-bold tracking-tight">
        Publish
      </h1>
      <p class="text-muted-foreground">
        Create and publish content to social media accounts
      </p>
    </div>

    <Card>
      <CardHeader>
        <CardTitle>New Publication</CardTitle>
      </CardHeader>
      <CardContent>
        <PublishForm
          :accounts="accountsData ?? []"
          :accounts-loading="accountsLoading"
          :publishing="publishing"
          @publish="handlePublish"
        />
      </CardContent>
    </Card>
  </div>
</template>
