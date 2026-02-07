import type { Component } from 'vue'
import { AlertTriangle, Database, MapPin, Pickaxe } from 'lucide-vue-next'
import { indexManagerApi } from '@/api/client'

export interface IntelligenceDrillDownItem {
  title: string
  description: string
  icon: Component
  route: string
  countKey?: string
}

export const INTELLIGENCE_DRILL_DOWNS: IntelligenceDrillDownItem[] = [
  {
    title: 'Crime Breakdown',
    description: 'Street crime relevance, types, and classification.',
    icon: AlertTriangle,
    route: '/intelligence/crime',
    countKey: 'crime',
  },
  {
    title: 'Mining Breakdown',
    description: 'Mining relevance, stage, commodities, and location.',
    icon: Pickaxe,
    route: '/intelligence/mining',
    countKey: 'mining',
  },
  {
    title: 'Location Breakdown',
    description: 'Geographic distribution by country, province, and city.',
    icon: MapPin,
    route: '/intelligence/location',
  },
  {
    title: 'Index Explorer',
    description: 'Browse classified content indexes and documents.',
    icon: Database,
    route: '/intelligence/indexes',
  },
]

export type CountFetcher = () => Promise<number>

export const COUNT_FETCHERS: Record<string, CountFetcher> = {
  crime: async () => {
    const res = await indexManagerApi.aggregations.getCrime()
    return res.data?.total_crime_related ?? 0
  },
  mining: async () => {
    const res = await indexManagerApi.aggregations.getMining()
    return res.data?.total_mining ?? 0
  },
}
