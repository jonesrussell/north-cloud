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
  // Smoke check: verifies rules don't fire when all inputs are nominal.
  // This does NOT prove the pipeline is healthy — it only validates rule logic.
  // See docs/TESTING_PIPELINE_CHECKLIST.md §1 for context.
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

describe('boundary and edge cases', () => {
  it('does not fire classification-backlog when backlog is at threshold (100)', () => {
    const metrics = healthyMetrics()
    metrics.indexes!.sources = [
      { source: 'threshold_src', rawCount: 200, classifiedCount: 100, backlog: 100, delta24h: 5, avgQuality: 70, active: true },
    ]
    const problems = detectProblems(metrics)
    expect(problems.find((p) => p.id === 'classification-backlog')).toBeUndefined()
  })

  it('fires classification-backlog when backlog is above threshold (101)', () => {
    const metrics = healthyMetrics()
    metrics.indexes!.sources = [
      { source: 'over_threshold', rawCount: 201, classifiedCount: 100, backlog: 101, delta24h: 5, avgQuality: 70, active: true },
    ]
    const problems = detectProblems(metrics)
    const p = problems.find((p) => p.id === 'classification-backlog')
    expect(p).toBeDefined()
    expect(p!.severity).toBe('warning')
  })

  it('fires failed-crawls with singular title for 1 failed job', () => {
    const metrics = healthyMetrics()
    metrics.crawler!.failedJobs = 1
    const problems = detectProblems(metrics)
    const p = problems.find((p) => p.id === 'failed-crawls')
    expect(p).toBeDefined()
    expect(p!.title).toBe('1 failed crawl job')
    expect(p!.count).toBe(1)
  })

  it('fires stale-scheduled-jobs with singular title for 1 stale job', () => {
    const metrics = healthyMetrics()
    metrics.crawler!.staleJobs = 1
    const problems = detectProblems(metrics)
    const p = problems.find((p) => p.id === 'stale-scheduled-jobs')
    expect(p).toBeDefined()
    expect(p!.title).toBe('1 stale scheduled job')
  })

  it('detects multiple simultaneous problems', () => {
    const metrics: PipelineMetrics = {
      crawler: { failedJobs: 5, staleJobs: 2, failedJobUrls: [] },
      indexes: {
        clusterHealth: 'yellow',
        sources: [
          { source: 'empty_active', rawCount: 0, classifiedCount: 0, backlog: 0, delta24h: 0, avgQuality: 0, active: true },
        ],
      },
      publisher: { publishedToday: 0, inactiveChannels: 1, inactiveChannelNames: ['Stale Channel'] },
    }
    const problems = detectProblems(metrics)
    const ids = problems.map((p) => p.id)
    expect(ids).toContain('failed-crawls')
    expect(ids).toContain('stale-scheduled-jobs')
    expect(ids).toContain('cluster-health')
    expect(ids).toContain('empty-indexes')
    expect(ids).toContain('zero-publishing')
    expect(ids).toContain('inactive-channels')
  })

  it('reports all three services unreachable with kind=system and severity=error', () => {
    const metrics: PipelineMetrics = {
      crawler: null,
      indexes: null,
      publisher: null,
    }
    const problems = detectProblems(metrics)
    expect(problems).toHaveLength(3)
    for (const p of problems) {
      expect(p.kind).toBe('system')
      expect(p.severity).toBe('error')
    }
  })
})
