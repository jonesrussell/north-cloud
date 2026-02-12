<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { AlertTriangle, Pickaxe, MapPin, BarChart3 } from 'lucide-vue-next'
import { Card, CardContent } from '@/components/ui/card'
import { indexManagerApi } from '@/api/client'

const router = useRouter()

interface SummaryCard {
  label: string
  icon: typeof AlertTriangle
  route?: string
  value: string
  sub: string
}

const cards = ref<SummaryCard[]>([
  { label: 'Crime', icon: AlertTriangle, route: '/dashboard/intelligence/crime', value: '-', sub: '' },
  { label: 'Mining', icon: Pickaxe, route: '/dashboard/intelligence/mining', value: '-', sub: '' },
  { label: 'Quality', icon: BarChart3, value: '-', sub: '' },
  { label: 'Location', icon: MapPin, value: '-', sub: '' },
])

onMounted(async () => {
  const [crimeRes, miningRes, overviewRes, locationRes] = await Promise.allSettled([
    indexManagerApi.aggregations.getCrime(),
    indexManagerApi.aggregations.getMining(),
    indexManagerApi.aggregations.getOverview(),
    indexManagerApi.aggregations.getLocation(),
  ])

  if (crimeRes.status === 'fulfilled') {
    const d = crimeRes.value.data
    cards.value[0].value = (d?.total_crime_related ?? 0).toLocaleString()
    const top = Object.entries(d?.by_sub_label ?? {})
      .sort(([, a], [, b]) => b - a)
      .slice(0, 3)
    cards.value[0].sub = top.map(([k]) => k.replace(/_/g, ' ')).join(', ')
  }

  if (miningRes.status === 'fulfilled') {
    const d = miningRes.value.data
    cards.value[1].value = (d?.total_mining ?? 0).toLocaleString()
    const top = Object.entries(d?.by_commodity ?? {})
      .sort(([, a], [, b]) => b - a)
      .slice(0, 3)
    cards.value[1].sub = top.map(([k]) => k).join(', ')
  }

  if (overviewRes.status === 'fulfilled') {
    const d = overviewRes.value.data
    const q = d?.quality_distribution
    if (q) {
      const total = (q.high ?? 0) + (q.medium ?? 0) + (q.low ?? 0)
      cards.value[2].value = total.toLocaleString()
      cards.value[2].sub = `${q.high ?? 0} high / ${q.medium ?? 0} med / ${q.low ?? 0} low`
    }
  }

  if (locationRes.status === 'fulfilled') {
    const d = locationRes.value.data
    const top = Object.entries(d?.by_city ?? {})
      .sort(([, a], [, b]) => b - a)
      .slice(0, 3)
    cards.value[3].value = top.length > 0 ? top[0][1].toLocaleString() : '-'
    cards.value[3].sub = top.map(([k]) => k).join(', ')
  }
})

function goTo(route?: string) {
  if (route) router.push(route)
}
</script>

<template>
  <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
    <Card
      v-for="card in cards"
      :key="card.label"
      class="transition-colors"
      :class="card.route ? 'cursor-pointer hover:bg-muted/50' : ''"
      @click="goTo(card.route)"
    >
      <CardContent class="pt-4 pb-3 px-4">
        <div class="flex items-center gap-2 mb-1">
          <component
            :is="card.icon"
            class="h-4 w-4 text-muted-foreground shrink-0"
          />
          <span class="text-xs font-medium uppercase tracking-wider text-muted-foreground">
            {{ card.label }}
          </span>
        </div>
        <p class="text-lg font-semibold tabular-nums">
          {{ card.value }}
        </p>
        <p
          v-if="card.sub"
          class="text-xs text-muted-foreground truncate mt-0.5"
        >
          {{ card.sub }}
        </p>
      </CardContent>
    </Card>
  </div>
</template>
