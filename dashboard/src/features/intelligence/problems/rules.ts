import type { PipelineMetrics, Problem } from './types'

const BACKLOG_THRESHOLD = 100

export function detectProblems(metrics: PipelineMetrics): Problem[] {
  const problems: Problem[] = []

  detectServiceUnreachable(metrics, problems)

  if (metrics.crawler) {
    detectCrawlerProblems(metrics.crawler, problems)
  }
  if (metrics.indexes) {
    detectIndexProblems(metrics.indexes, problems)
  }
  if (metrics.publisher) {
    detectPublisherProblems(metrics.publisher, problems)
  }

  return problems
}

function detectServiceUnreachable(metrics: PipelineMetrics, problems: Problem[]): void {
  const services = [
    { key: 'crawler' as const, label: 'Crawler' },
    { key: 'indexes' as const, label: 'Index manager' },
    { key: 'publisher' as const, label: 'Publisher' },
  ]
  for (const svc of services) {
    if (metrics[svc.key] === null) {
      problems.push({
        id: `service-unreachable-${svc.key}`,
        kind: 'system',
        severity: 'error',
        title: `${svc.label} metrics unavailable`,
        action: `${svc.label} service may be down or auth misconfigured. Check service health and logs.`,
      })
    }
  }
}

function detectCrawlerProblems(
  crawler: NonNullable<PipelineMetrics['crawler']>,
  problems: Problem[],
): void {
  if (crawler.failedJobs > 0) {
    problems.push({
      id: 'failed-crawls',
      kind: 'crawler',
      severity: 'error',
      title: `${crawler.failedJobs} failed crawl job${crawler.failedJobs === 1 ? '' : 's'}`,
      action: 'Open job details, check last error, consider disabling or fixing source config.',
      link: '/jobs?status=failed',
      count: crawler.failedJobs,
    })
  }
  if (crawler.staleJobs > 0) {
    problems.push({
      id: 'stale-scheduled-jobs',
      kind: 'crawler',
      severity: 'error',
      title: `${crawler.staleJobs} stale scheduled job${crawler.staleJobs === 1 ? '' : 's'}`,
      action: 'Crawler scheduler may be down. Check service health.',
      link: '/jobs',
      count: crawler.staleJobs,
    })
  }
}

function detectIndexProblems(
  indexes: NonNullable<PipelineMetrics['indexes']>,
  problems: Problem[],
): void {
  if (indexes.clusterHealth !== 'green') {
    problems.push({
      id: 'cluster-health',
      kind: 'system',
      severity: indexes.clusterHealth === 'red' ? 'error' : 'warning',
      title: `Elasticsearch cluster health: ${indexes.clusterHealth}`,
      action: 'Check Elasticsearch cluster status and shard allocation.',
    })
  }

  const activeSources = indexes.sources.filter((s) => s.active)
  const emptySources = activeSources.filter((s) => s.classifiedCount === 0)
  if (emptySources.length > 0) {
    problems.push({
      id: 'empty-indexes',
      kind: 'index',
      severity: 'warning',
      title: `${emptySources.length} active source${emptySources.length === 1 ? '' : 's'} with no classified content`,
      action: 'Verify crawler is configured and running for these sources.',
      link: '/intelligence/indexes',
      count: emptySources.length,
      sourceIds: emptySources.map((s) => s.source),
    })
  }

  const backlogSources = activeSources.filter((s) => s.backlog > BACKLOG_THRESHOLD)
  if (backlogSources.length > 0) {
    problems.push({
      id: 'classification-backlog',
      kind: 'index',
      severity: 'warning',
      title: `${backlogSources.length} source${backlogSources.length === 1 ? '' : 's'} with classification backlog`,
      action: 'Classifier may be stalled or slow. Check service logs.',
      link: '/intelligence',
      count: backlogSources.length,
      sourceIds: backlogSources.map((s) => s.source),
    })
  }
}

function detectPublisherProblems(
  publisher: NonNullable<PipelineMetrics['publisher']>,
  problems: Problem[],
): void {
  if (publisher.inactiveChannels > 0) {
    problems.push({
      id: 'inactive-channels',
      kind: 'publisher',
      severity: 'warning',
      title: `${publisher.inactiveChannels} inactive channel${publisher.inactiveChannels === 1 ? '' : 's'}`,
      action: `Enable or remove: ${publisher.inactiveChannelNames.join(', ')}.`,
      link: '/channels',
      count: publisher.inactiveChannels,
    })
  }
  if (publisher.publishedToday === 0) {
    problems.push({
      id: 'zero-publishing',
      kind: 'publisher',
      severity: 'error',
      title: 'No articles published today',
      action: 'Check channel status, route configuration, and classified content availability.',
      link: '/channels',
    })
  }
}
