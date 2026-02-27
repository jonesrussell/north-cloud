<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import {
  ArrowLeft,
  Loader2,
  Link as LinkIcon,
  CheckCircle,
  FileCode,
  Layers,
  Globe,
  Plus,
  ClipboardCheck,
  EyeOff,
} from 'lucide-vue-next'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { DataTablePagination } from '@/components/common'
import { crawlerApi } from '@/api/client'
import { formatRelativeTime } from '@/lib/utils'
import {
  fetchDomainDetail,
  fetchDomainLinks,
  discoveredDomainsKeys,
} from '@/features/intake/api/discoveredDomains'
import type {
  DiscoveredDomainLink,
  PathCluster,
} from '@/features/intake/api/discoveredDomains'

const route = useRoute()
const router = useRouter()
const queryClient = useQueryClient()

const domain = computed(() => decodeURIComponent(route.params.domain as string))

// --- Links pagination ---

const linksPage = ref(1)
const linksPageSize = ref(25)
const linksOffset = computed(() => (linksPage.value - 1) * linksPageSize.value)

// --- Data fetching ---

const {
  data: domainDetail,
  isLoading: detailLoading,
  error: detailError,
} = useQuery({
  queryKey: computed(() => discoveredDomainsKeys.detail(domain.value)),
  queryFn: () => fetchDomainDetail(domain.value),
})

const {
  data: linksData,
  isLoading: linksLoading,
} = useQuery({
  queryKey: computed(() => [...discoveredDomainsKeys.links(domain.value), linksPage.value, linksPageSize.value]),
  queryFn: () => fetchDomainLinks(domain.value, { limit: linksPageSize.value, offset: linksOffset.value }),
})

const links = computed<DiscoveredDomainLink[]>(() => linksData.value?.links ?? [])
const pathClusters = computed<PathCluster[]>(() => {
  const clusters = linksData.value?.path_clusters ?? []
  return [...clusters].sort((a, b) => b.count - a.count)
})
const linksTotal = computed(() => linksData.value?.total ?? 0)
const linksTotalPages = computed(() => Math.max(1, Math.ceil(linksTotal.value / linksPageSize.value)))

const ALLOWED_PAGE_SIZES = [10, 25, 50, 100] as const

function setLinksPage(page: number) {
  linksPage.value = page
}

function setLinksPageSize(size: number) {
  linksPageSize.value = size
  linksPage.value = 1
}

// --- Status helpers ---

type StatusVariant = 'info' | 'warning' | 'pending' | 'success' | 'outline'

function getStatusVariant(status: string): StatusVariant {
  switch (status) {
    case 'active': return 'info'
    case 'reviewing': return 'warning'
    case 'ignored': return 'pending'
    case 'promoted': return 'success'
    default: return 'outline'
  }
}

type ScoreVariant = 'success' | 'warning' | 'destructive'

const HIGH_SCORE_THRESHOLD = 70
const MID_SCORE_THRESHOLD = 40

function getScoreVariant(score: number): ScoreVariant {
  if (score >= HIGH_SCORE_THRESHOLD) return 'success'
  if (score >= MID_SCORE_THRESHOLD) return 'warning'
  return 'destructive'
}

type HttpStatusVariant = 'success' | 'warning' | 'destructive' | 'pending'

const HTTP_STATUS_OK_MIN = 200
const HTTP_STATUS_OK_MAX = 299
const HTTP_STATUS_CLIENT_ERROR_MIN = 400
const HTTP_STATUS_CLIENT_ERROR_MAX = 499

function getHttpStatusVariant(status: number | null): HttpStatusVariant {
  if (status === null) return 'pending'
  if (status >= HTTP_STATUS_OK_MIN && status <= HTTP_STATUS_OK_MAX) return 'success'
  if (status >= HTTP_STATUS_CLIENT_ERROR_MIN && status <= HTTP_STATUS_CLIENT_ERROR_MAX) return 'warning'
  return 'destructive'
}

function formatPercent(value: number | null): string {
  if (value === null || value === undefined) return '\u2014'
  return `${Math.round(value * 100)}%`
}

// --- Actions ---

async function updateStatus(status: string) {
  try {
    await crawlerApi.discoveredDomains.updateState(domain.value, { status })
    queryClient.invalidateQueries({ queryKey: ['discovered-domains'] })
  } catch (err: unknown) {
    console.error('Error updating domain status:', err)
  }
}

function createSource() {
  router.push(`/sources/new?domain=${encodeURIComponent(domain.value)}&name=${encodeURIComponent(domain.value)}`)
}

function goBack() {
  router.push('/intake/discovered-links')
}
</script>

<template>
  <div class="space-y-6">
    <!-- Back link -->
    <Button
      variant="ghost"
      size="sm"
      @click="goBack"
    >
      <ArrowLeft class="mr-2 h-4 w-4" />
      Back to Discovered Domains
    </Button>

    <!-- Loading -->
    <div
      v-if="detailLoading"
      class="flex items-center justify-center py-12"
    >
      <Loader2 class="h-8 w-8 animate-spin text-muted-foreground" />
    </div>

    <!-- Error -->
    <Card
      v-else-if="detailError"
      class="border-destructive"
    >
      <CardContent class="pt-6">
        <p class="text-destructive">
          {{ (detailError as Error)?.message || 'Unable to load domain details.' }}
        </p>
      </CardContent>
    </Card>

    <!-- Content -->
    <template v-else-if="domainDetail">
      <!-- Header row -->
      <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div class="flex items-center gap-3">
          <h1 class="text-3xl font-bold tracking-tight">
            {{ domainDetail.domain }}
          </h1>
          <Badge :variant="getScoreVariant(domainDetail.quality_score)">
            Score: {{ domainDetail.quality_score }}
          </Badge>
          <Badge :variant="getStatusVariant(domainDetail.status)">
            {{ domainDetail.status }}
          </Badge>
        </div>
        <div class="flex items-center gap-2">
          <Button @click="createSource">
            <Plus class="mr-2 h-4 w-4" />
            Create Source
          </Button>
          <Button
            variant="outline"
            @click="updateStatus('reviewing')"
          >
            <ClipboardCheck class="mr-2 h-4 w-4" />
            Mark Reviewing
          </Button>
          <Button
            variant="outline"
            @click="updateStatus('ignored')"
          >
            <EyeOff class="mr-2 h-4 w-4" />
            Ignore
          </Button>
        </div>
      </div>

      <!-- Summary cards -->
      <div class="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-5">
        <Card>
          <CardContent class="flex flex-col items-center justify-center pt-6">
            <LinkIcon class="h-5 w-5 text-muted-foreground mb-1" />
            <p class="text-2xl font-bold">
              {{ domainDetail.link_count }}
            </p>
            <p class="text-xs text-muted-foreground">
              Total Links
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardContent class="flex flex-col items-center justify-center pt-6">
            <CheckCircle class="h-5 w-5 text-muted-foreground mb-1" />
            <p class="text-2xl font-bold">
              {{ formatPercent(domainDetail.ok_ratio) }}
            </p>
            <p class="text-xs text-muted-foreground">
              OK Rate
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardContent class="flex flex-col items-center justify-center pt-6">
            <FileCode class="h-5 w-5 text-muted-foreground mb-1" />
            <p class="text-2xl font-bold">
              {{ formatPercent(domainDetail.html_ratio) }}
            </p>
            <p class="text-xs text-muted-foreground">
              HTML Rate
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardContent class="flex flex-col items-center justify-center pt-6">
            <Layers class="h-5 w-5 text-muted-foreground mb-1" />
            <p class="text-2xl font-bold">
              {{ domainDetail.avg_depth.toFixed(1) }}
            </p>
            <p class="text-xs text-muted-foreground">
              Avg Depth
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardContent class="flex flex-col items-center justify-center pt-6">
            <Globe class="h-5 w-5 text-muted-foreground mb-1" />
            <p class="text-2xl font-bold">
              {{ domainDetail.source_count }}
            </p>
            <p class="text-xs text-muted-foreground">
              Sources
            </p>
          </CardContent>
        </Card>
      </div>

      <!-- Path Clusters -->
      <Card v-if="pathClusters.length > 0">
        <CardHeader>
          <CardTitle>Path Clusters</CardTitle>
        </CardHeader>
        <CardContent class="p-0">
          <div class="rounded-md border">
            <table class="w-full">
              <thead>
                <tr class="border-b bg-muted/50">
                  <th class="px-4 py-3 text-left text-sm font-medium text-muted-foreground">
                    Pattern
                  </th>
                  <th class="px-4 py-3 text-left text-sm font-medium text-muted-foreground">
                    Count
                  </th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="cluster in pathClusters"
                  :key="cluster.pattern"
                  class="border-b transition-colors hover:bg-muted/50"
                >
                  <td class="px-4 py-3 text-sm font-mono">
                    {{ cluster.pattern }}
                  </td>
                  <td class="px-4 py-3 text-sm text-muted-foreground">
                    {{ cluster.count }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>

      <!-- URLs Table -->
      <Card>
        <CardHeader>
          <CardTitle>URLs</CardTitle>
        </CardHeader>
        <CardContent class="p-0">
          <div class="space-y-4">
            <div class="rounded-md border">
              <table class="w-full">
                <thead>
                  <tr class="border-b bg-muted/50">
                    <th class="px-4 py-3 text-left text-sm font-medium text-muted-foreground">
                      Path
                    </th>
                    <th class="px-4 py-3 text-left text-sm font-medium text-muted-foreground">
                      HTTP Status
                    </th>
                    <th class="px-4 py-3 text-left text-sm font-medium text-muted-foreground">
                      Content Type
                    </th>
                    <th class="px-4 py-3 text-left text-sm font-medium text-muted-foreground">
                      Depth
                    </th>
                    <th class="px-4 py-3 text-left text-sm font-medium text-muted-foreground">
                      Source
                    </th>
                    <th class="px-4 py-3 text-left text-sm font-medium text-muted-foreground">
                      Discovered
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <!-- Loading skeletons -->
                  <template v-if="linksLoading">
                    <tr
                      v-for="i in 5"
                      :key="i"
                      class="border-b"
                    >
                      <td
                        v-for="j in 6"
                        :key="j"
                        class="px-4 py-3"
                      >
                        <Skeleton class="h-4 w-24" />
                      </td>
                    </tr>
                  </template>

                  <!-- Empty state -->
                  <tr
                    v-else-if="links.length === 0"
                    class="border-b"
                  >
                    <td
                      colspan="6"
                      class="px-4 py-12 text-center"
                    >
                      <p class="text-sm text-muted-foreground">
                        No URLs found for this domain
                      </p>
                    </td>
                  </tr>

                  <!-- Data rows -->
                  <tr
                    v-for="link in links"
                    v-else
                    :key="link.id"
                    class="border-b transition-colors hover:bg-muted/50"
                  >
                    <td class="px-4 py-3">
                      <a
                        :href="link.url"
                        target="_blank"
                        rel="noopener noreferrer"
                        class="text-sm text-primary hover:underline truncate block max-w-md font-mono"
                        @click.stop
                      >
                        {{ link.path || '/' }}
                      </a>
                    </td>
                    <td class="px-4 py-3">
                      <Badge
                        v-if="link.http_status !== null"
                        :variant="getHttpStatusVariant(link.http_status)"
                      >
                        {{ link.http_status }}
                      </Badge>
                      <span
                        v-else
                        class="text-sm text-muted-foreground"
                      >
                        &mdash;
                      </span>
                    </td>
                    <td class="px-4 py-3 text-sm text-muted-foreground">
                      {{ link.content_type || '\u2014' }}
                    </td>
                    <td class="px-4 py-3 text-sm text-muted-foreground">
                      {{ link.depth }}
                    </td>
                    <td class="px-4 py-3 text-sm text-muted-foreground">
                      {{ link.source_name }}
                    </td>
                    <td class="px-4 py-3 text-sm text-muted-foreground">
                      {{ formatRelativeTime(link.discovered_at) }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>

            <DataTablePagination
              :page="linksPage"
              :page-size="linksPageSize"
              :total="linksTotal"
              :total-pages="linksTotalPages"
              :allowed-page-sizes="ALLOWED_PAGE_SIZES"
              item-label="URLs"
              @update:page="setLinksPage"
              @update:page-size="setLinksPageSize"
            />
          </div>
        </CardContent>
      </Card>
    </template>
  </div>
</template>
