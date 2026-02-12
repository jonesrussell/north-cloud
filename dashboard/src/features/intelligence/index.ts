// API & data
export { fetchIndexes, fetchIndexStats, deleteIndex, indexesKeys } from './api/indexes'
export type { IndexFilters } from './api/indexes'

// Composables
export { useIndexes } from './composables/useIndexes'
export { usePipelineHealth } from './composables/usePipelineHealth'

// Problem detection
export { detectProblems } from './problems/rules'
export type { Problem, PipelineMetrics } from './problems/types'

// Components
export { default as IndexesFilterBar } from './components/IndexesFilterBar.vue'
export { default as IndexesTable } from './components/IndexesTable.vue'
export { default as IndexStatsCards } from './components/IndexStatsCards.vue'
export { default as PipelineKPIs } from './components/PipelineKPIs.vue'
export { default as SourceHealthTable } from './components/SourceHealthTable.vue'
export { default as ContentSummaryCards } from './components/ContentSummaryCards.vue'
