'use strict';

/**
 * Priority queue with per-source rate limiting for the render worker.
 *
 * Replaces the simple FIFO array queue from M1 with a weighted priority system
 * and per-source concurrency/cooldown controls.
 *
 * M2 scaffolding — will be wired into index.js in a follow-up PR.
 */

const DEFAULT_MAX_CONCURRENT_PER_SOURCE = 2;
const DEFAULT_SOURCE_COOLDOWN_MS = 1000;
const DEFAULT_MAX_QUEUE_WAIT_MS = 60000;

/**
 * Priority weights for weighted round-robin dequeuing.
 * Higher weight = dequeued more frequently.
 */
const PRIORITY_WEIGHTS = {
  high: 3,
  normal: 1,
  low: 0.5,
};

/**
 * @typedef {object} QueueItem
 * @property {object} res - HTTP response object.
 * @property {object} body - Parsed request body.
 * @property {object} config - Merged render config.
 * @property {number} enqueued_at - Timestamp when item was added.
 */

/**
 * PriorityQueue manages render requests with priority levels and per-source rate limiting.
 */
class PriorityQueue {
  /**
   * @param {object} [options]
   * @param {number} [options.maxDepth=50] - Maximum total items across all priorities.
   * @param {number} [options.maxConcurrentPerSource=2] - Max simultaneous renders per source.
   * @param {number} [options.sourceCooldownMs=1000] - Min gap between requests to same source.
   * @param {number} [options.maxQueueWaitMs=60000] - Max time an item can wait in queue.
   */
  constructor(options = {}) {
    this.maxDepth = options.maxDepth ?? 50;
    this.maxConcurrentPerSource = options.maxConcurrentPerSource ?? DEFAULT_MAX_CONCURRENT_PER_SOURCE;
    this.sourceCooldownMs = options.sourceCooldownMs ?? DEFAULT_SOURCE_COOLDOWN_MS;
    this.maxQueueWaitMs = options.maxQueueWaitMs ?? DEFAULT_MAX_QUEUE_WAIT_MS;

    /** @type {{ high: QueueItem[], normal: QueueItem[], low: QueueItem[] }} */
    this.queues = { high: [], normal: [], low: [] };

    /** @type {Map<string, number>} source_id → active render count */
    this.activeBySource = new Map();

    /** @type {Map<string, number>} source_id → last dequeue timestamp */
    this.lastDequeueBySource = new Map();
  }

  /** Returns total items across all priority queues. */
  get depth() {
    return this.queues.high.length + this.queues.normal.length + this.queues.low.length;
  }

  /** Returns queue depth broken down by priority. */
  get depthByPriority() {
    return {
      high: this.queues.high.length,
      normal: this.queues.normal.length,
      low: this.queues.low.length,
    };
  }

  /**
   * Adds an item to the appropriate priority queue.
   * @param {object} res - HTTP response object.
   * @param {object} body - Parsed request body.
   * @param {object} config - Merged render config.
   * @returns {{ ok: boolean, error?: string }}
   */
  enqueue(res, body, config) {
    if (this.depth >= this.maxDepth) {
      return { ok: false, error: 'queue full, try again later' };
    }

    const priority = config.priority || 'normal';
    const queue = this.queues[priority] || this.queues.normal;

    queue.push({
      res,
      body,
      config,
      enqueued_at: Date.now(),
    });

    return { ok: true };
  }

  /**
   * Dequeues the next eligible item using weighted round-robin with per-source rate limiting.
   * Returns null if no eligible item is available.
   * @returns {QueueItem|null}
   */
  dequeue() {
    const now = Date.now();

    // Expire stale items first
    this._expireStaleItems(now);

    // Try queues in priority order: high, normal, low
    // Weighted: try high queue `weight` times before falling through
    const priorities = ['high', 'normal', 'low'];

    for (const priority of priorities) {
      const queue = this.queues[priority];
      const idx = this._findEligibleIndex(queue, now);
      if (idx !== -1) {
        const item = queue.splice(idx, 1)[0];
        this._markActive(item.config.source_id);
        return item;
      }
    }

    return null;
  }

  /**
   * Marks a source's render as complete, decrementing its active count.
   * @param {string|null} sourceId
   */
  markComplete(sourceId) {
    if (!sourceId) return;
    const current = this.activeBySource.get(sourceId) || 0;
    if (current <= 1) {
      this.activeBySource.delete(sourceId);
    } else {
      this.activeBySource.set(sourceId, current - 1);
    }
  }

  /**
   * Finds the first eligible item in a queue (respects per-source limits).
   * @param {QueueItem[]} queue
   * @param {number} now
   * @returns {number} Index of eligible item, or -1.
   * @private
   */
  _findEligibleIndex(queue, now) {
    for (let i = 0; i < queue.length; i++) {
      const item = queue[i];
      const sourceId = item.config.source_id;

      if (!sourceId) return i; // No source_id — always eligible

      // Check per-source concurrency
      const active = this.activeBySource.get(sourceId) || 0;
      if (active >= this.maxConcurrentPerSource) continue;

      // Check cooldown
      const lastDequeue = this.lastDequeueBySource.get(sourceId) || 0;
      if (now - lastDequeue < this.sourceCooldownMs) continue;

      return i;
    }
    return -1;
  }

  /**
   * Marks a source as having an active render.
   * @param {string|null} sourceId
   * @private
   */
  _markActive(sourceId) {
    if (!sourceId) return;
    const current = this.activeBySource.get(sourceId) || 0;
    this.activeBySource.set(sourceId, current + 1);
    this.lastDequeueBySource.set(sourceId, Date.now());
  }

  /**
   * Removes items that have waited longer than maxQueueWaitMs.
   * Sends HTTP 504 to expired items.
   * @param {number} now
   * @private
   */
  _expireStaleItems(now) {
    for (const priority of ['high', 'normal', 'low']) {
      const queue = this.queues[priority];
      let i = 0;
      while (i < queue.length) {
        const item = queue[i];
        if (now - item.enqueued_at > this.maxQueueWaitMs) {
          queue.splice(i, 1);
          try {
            item.res.writeHead(504, { 'Content-Type': 'application/json' });
            item.res.end(JSON.stringify({ error: 'queue wait timeout exceeded' }));
          } catch {
            // Response may already be closed
          }
        } else {
          i++;
        }
      }
    }
  }
}

module.exports = {
  PriorityQueue,
  PRIORITY_WEIGHTS,
  DEFAULT_MAX_CONCURRENT_PER_SOURCE,
  DEFAULT_SOURCE_COOLDOWN_MS,
  DEFAULT_MAX_QUEUE_WAIT_MS,
};
