import { describe, it, expect } from 'vitest'
import { detectProblems } from './rules'
import type { PipelineMetrics } from './types'

function healthyMetrics(): PipelineMetrics {
  return {
    crawler: { failedJobs: 0, staleJobs: 0, failedJobUrls: [] },
    indexes: {
      clusterHealth: 'green',
      sources: [
        { source: 'example_com', rawCount: 100, classifiedCount: 95, backlog: 5, delta24h: 10, avgQuality: 72, active: true },
      ],
    },
    publisher: { publishedToday: 42, inactiveChannels: 0, inactiveChannelNames: [] },
  }
}

describe('detectProblems', () => {
  it('returns empty array when everything is healthy', () => {
    const problems = detectProblems(healthyMetrics())
    expect(problems).toEqual([])
  })

  it('detects failed crawl jobs', () => {
    const metrics = healthyMetrics()
    metrics.crawler!.failedJobs = 18
    const problems = detectProblems(metrics)
    const p = problems.find((p) => p.id === 'failed-crawls')
    expect(p).toBeDefined()
    expect(p!.severity).toBe('error')
    expect(p!.kind).toBe('crawler')
    expect(p!.count).toBe(18)
  })

  it('detects stale scheduled jobs', () => {
    const metrics = healthyMetrics()
    metrics.crawler!.staleJobs = 3
    const problems = detectProblems(metrics)
    const p = problems.find((p) => p.id === 'stale-scheduled-jobs')
    expect(p).toBeDefined()
    expect(p!.severity).toBe('error')
  })

  it('detects empty indexes for active sources', () => {
    const metrics = healthyMetrics()
    metrics.indexes!.sources = [
      { source: 'dead_source', rawCount: 0, classifiedCount: 0, backlog: 0, delta24h: 0, avgQuality: 0, active: true },
    ]
    const problems = detectProblems(metrics)
    const p = problems.find((p) => p.id === 'empty-indexes')
    expect(p).toBeDefined()
    expect(p!.severity).toBe('warning')
    expect(p!.count).toBe(1)
  })

  it('ignores empty indexes for inactive sources', () => {
    const metrics = healthyMetrics()
    metrics.indexes!.sources = [
      { source: 'paused', rawCount: 0, classifiedCount: 0, backlog: 0, delta24h: 0, avgQuality: 0, active: false },
    ]
    const problems = detectProblems(metrics)
    expect(problems.find((p) => p.id === 'empty-indexes')).toBeUndefined()
  })

  it('detects classification backlog', () => {
    const metrics = healthyMetrics()
    metrics.indexes!.sources = [
      { source: 'backed_up', rawCount: 500, classifiedCount: 100, backlog: 400, delta24h: 0, avgQuality: 60, active: true },
    ]
    const problems = detectProblems(metrics)
    const p = problems.find((p) => p.id === 'classification-backlog')
    expect(p).toBeDefined()
    expect(p!.severity).toBe('warning')
  })

  it('detects inactive channels', () => {
    const metrics = healthyMetrics()
    metrics.publisher!.inactiveChannels = 2
    metrics.publisher!.inactiveChannelNames = ['Crime Feed', 'Mining Feed']
    const problems = detectProblems(metrics)
    const p = problems.find((p) => p.id === 'inactive-channels')
    expect(p).toBeDefined()
    expect(p!.severity).toBe('warning')
    expect(p!.count).toBe(2)
  })

  it('detects zero publishing', () => {
    const metrics = healthyMetrics()
    metrics.publisher!.publishedToday = 0
    const problems = detectProblems(metrics)
    const p = problems.find((p) => p.id === 'zero-publishing')
    expect(p).toBeDefined()
    expect(p!.severity).toBe('error')
  })

  it('detects degraded cluster health', () => {
    const metrics = healthyMetrics()
    metrics.indexes!.clusterHealth = 'yellow'
    const problems = detectProblems(metrics)
    const p = problems.find((p) => p.id === 'cluster-health')
    expect(p).toBeDefined()
    expect(p!.severity).toBe('warning')
  })

  it('detects red cluster health as error', () => {
    const metrics = healthyMetrics()
    metrics.indexes!.clusterHealth = 'red'
    const problems = detectProblems(metrics)
    const p = problems.find((p) => p.id === 'cluster-health')
    expect(p).toBeDefined()
    expect(p!.severity).toBe('error')
  })

  it('detects unreachable crawler service', () => {
    const metrics = healthyMetrics()
    metrics.crawler = null
    const problems = detectProblems(metrics)
    const p = problems.find((p) => p.id === 'service-unreachable-crawler')
    expect(p).toBeDefined()
    expect(p!.severity).toBe('error')
    expect(p!.kind).toBe('system')
  })

  it('detects unreachable publisher service', () => {
    const metrics = healthyMetrics()
    metrics.publisher = null
    const problems = detectProblems(metrics)
    const p = problems.find((p) => p.id === 'service-unreachable-publisher')
    expect(p).toBeDefined()
  })

  it('detects unreachable index-manager service', () => {
    const metrics = healthyMetrics()
    metrics.indexes = null
    const problems = detectProblems(metrics)
    const p = problems.find((p) => p.id === 'service-unreachable-indexes')
    expect(p).toBeDefined()
  })
})
