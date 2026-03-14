'use strict';

const { test } = require('node:test');
const assert = require('node:assert/strict');
const http = require('node:http');
const { createRequestHandler, MAX_QUEUE_DEPTH } = require('./handler');
const { STEALTH_INIT_SCRIPT } = require('./stealth');
const { mergeConfig, validateConfig, DEFAULT_CONFIG, SCROLL_STRATEGIES, PRIORITY_LEVELS } = require('./config');
const { PriorityQueue } = require('./queue');
const { scrollViewport, STRATEGY_MAP } = require('./scroll');

// Build a minimal state object and no-op processQueue for tests that don't need rendering.
function makeState(overrides) {
  return {
    browser: null,
    requestCount: 0,
    queueDepth: 0,
    queue: [],
    recycleAfter: 100,
    ...overrides,
  };
}

// Helper: start a test server with the given state and processQueue stub.
function startServer(state, processQueue) {
  const handler = createRequestHandler(state, processQueue || (() => {}));
  const server = http.createServer(handler);
  return new Promise((resolve) => server.listen(0, '127.0.0.1', () => resolve(server)));
}

// Helper: make an HTTP request, returns { status, body }.
function request(server, method, path, payload) {
  return new Promise((resolve, reject) => {
    const addr = server.address();
    const body = payload !== undefined ? JSON.stringify(payload) : undefined;
    const opts = {
      host: '127.0.0.1',
      port: addr.port,
      method,
      path,
      headers: body ? { 'Content-Type': 'application/json', 'Content-Length': Buffer.byteLength(body) } : {},
    };
    const req = http.request(opts, (res) => {
      let data = '';
      res.on('data', (chunk) => { data += chunk; });
      res.on('end', () => {
        try { resolve({ status: res.statusCode, body: JSON.parse(data) }); }
        catch { resolve({ status: res.statusCode, body: data }); }
      });
    });
    req.on('error', reject);
    if (body) req.write(body);
    req.end();
  });
}

// Helper: send raw bytes as POST /render body.
function requestRaw(server, rawBody) {
  return new Promise((resolve, reject) => {
    const addr = server.address();
    const opts = {
      host: '127.0.0.1',
      port: addr.port,
      method: 'POST',
      path: '/render',
      headers: { 'Content-Type': 'application/json', 'Content-Length': Buffer.byteLength(rawBody) },
    };
    const req = http.request(opts, (res) => {
      let data = '';
      res.on('data', (c) => { data += c; });
      res.on('end', () => {
        try { resolve({ status: res.statusCode, body: JSON.parse(data) }); }
        catch { resolve({ status: res.statusCode, body: data }); }
      });
    });
    req.on('error', reject);
    req.write(rawBody);
    req.end();
  });
}

// ─── Handler Tests (M1 — unchanged) ────────────────────────────────────────

test('GET /health returns ok status with expected fields', async () => {
  const state = makeState({ requestCount: 5, queueDepth: 2, recycleAfter: 50 });
  const server = await startServer(state);
  try {
    const { status, body } = await request(server, 'GET', '/health');
    assert.equal(status, 200);
    assert.equal(body.status, 'ok');
    assert.equal(body.browser_connected, false);
    assert.equal(body.request_count, 5);
    assert.equal(body.queue_depth, 2);
    assert.equal(body.recycle_after, 50);
  } finally {
    server.close();
  }
});

test('GET /unknown returns 404', async () => {
  const server = await startServer(makeState());
  try {
    const { status, body } = await request(server, 'GET', '/unknown');
    assert.equal(status, 404);
    assert.equal(body.error, 'not found');
  } finally {
    server.close();
  }
});

test('POST /render with missing url returns 400', async () => {
  const server = await startServer(makeState());
  try {
    const { status, body } = await request(server, 'POST', '/render', { wait_until: 'load' });
    assert.equal(status, 400);
    assert.equal(body.error, 'url is required');
  } finally {
    server.close();
  }
});

test('POST /render with invalid JSON returns 400', async () => {
  const server = await startServer(makeState());
  try {
    const { status, body } = await requestRaw(server, '{not valid json}');
    assert.equal(status, 400);
    assert.equal(body.error, 'invalid JSON');
  } finally {
    server.close();
  }
});

test('POST /render with full queue returns 503', async () => {
  const state = makeState();
  // Pre-fill queue to capacity with dummy entries.
  for (let i = 0; i < MAX_QUEUE_DEPTH; i++) {
    state.queue.push({ res: null, body: { url: `http://example.com/${i}` } });
  }
  const server = await startServer(state);
  try {
    const { status, body } = await request(server, 'POST', '/render', { url: 'http://example.com/new' });
    assert.equal(status, 503);
    assert.equal(body.error, 'queue full, try again later');
  } finally {
    server.close();
  }
});

test('STEALTH_INIT_SCRIPT patches navigator.webdriver, plugins, and languages', () => {
  const vm = require('node:vm');

  // Run the stealth script in an isolated vm context with a fake navigator object.
  const ctx = { navigator: {}, Object };
  vm.createContext(ctx);
  vm.runInContext(STEALTH_INIT_SCRIPT, ctx);

  // navigator.webdriver should be undefined (not true)
  assert.equal(Object.getOwnPropertyDescriptor(ctx.navigator, 'webdriver').get(), undefined);

  // navigator.plugins should have at least one entry
  const plugins = Object.getOwnPropertyDescriptor(ctx.navigator, 'plugins').get();
  assert.ok(plugins.length > 0, 'plugins.length should be > 0');

  // navigator.languages should be a non-empty array
  const languages = Object.getOwnPropertyDescriptor(ctx.navigator, 'languages').get();
  assert.ok(Array.isArray(languages) && languages.length > 0, 'languages should be non-empty array');
});

// ─── Handler Tests (M2 — config parsing) ───────────────────────────────────

test('POST /render with valid config enqueues item with merged config', async () => {
  const state = makeState();
  let enqueuedConfig = null;
  // processQueue drains the item and sends a response so the HTTP request completes.
  const processQueue = () => {
    if (state.queue.length > 0) {
      const item = state.queue.shift();
      enqueuedConfig = item.config;
      item.res.writeHead(200, { 'Content-Type': 'application/json' });
      item.res.end(JSON.stringify({ html: '', final_url: '', render_time_ms: 0, status_code: 200 }));
    }
  };
  const server = await startServer(state, processQueue);
  try {
    const { status } = await request(server, 'POST', '/render', {
      url: 'http://example.com',
      config: { scroll: { strategy: 'full_page' }, priority: 'high' },
    });
    assert.equal(status, 200);
    assert.ok(enqueuedConfig, 'config should be present on enqueued item');
    assert.equal(enqueuedConfig.scroll.strategy, 'full_page');
    assert.equal(enqueuedConfig.priority, 'high');
    // Defaults applied
    assert.equal(enqueuedConfig.scroll.max_scroll_ms, 10000);
    assert.equal(enqueuedConfig.viewport.width, 1280);
  } finally {
    server.close();
  }
});

test('POST /render without config enqueues with defaults (backwards compatible)', async () => {
  const state = makeState();
  let enqueuedConfig = null;
  const processQueue = () => {
    if (state.queue.length > 0) {
      const item = state.queue.shift();
      enqueuedConfig = item.config;
      item.res.writeHead(200, { 'Content-Type': 'application/json' });
      item.res.end(JSON.stringify({ html: '', final_url: '', render_time_ms: 0, status_code: 200 }));
    }
  };
  const server = await startServer(state, processQueue);
  try {
    await request(server, 'POST', '/render', { url: 'http://example.com' });
    assert.ok(enqueuedConfig, 'config should be present with defaults');
    assert.equal(enqueuedConfig.scroll.strategy, 'viewport');
    assert.equal(enqueuedConfig.priority, 'normal');
    assert.equal(enqueuedConfig.source_id, null);
  } finally {
    server.close();
  }
});

test('POST /render with invalid scroll config returns 400', async () => {
  const server = await startServer(makeState());
  try {
    const { status, body } = await request(server, 'POST', '/render', {
      url: 'http://example.com',
      config: { scroll: { strategy: 'pixels', pixels: 0 } },
    });
    assert.equal(status, 400);
    assert.ok(body.error.includes('pixels'));
  } finally {
    server.close();
  }
});

// ─── Config Tests ───────────────────────────────────────────────────────────

test('mergeConfig returns defaults when called with undefined', () => {
  const config = mergeConfig(undefined);
  assert.deepStrictEqual(config.scroll, DEFAULT_CONFIG.scroll);
  assert.equal(config.priority, 'normal');
  assert.equal(config.source_id, null);
});

test('mergeConfig returns defaults when called with null', () => {
  const config = mergeConfig(null);
  assert.equal(config.scroll.strategy, 'viewport');
});

test('mergeConfig merges partial scroll config', () => {
  const config = mergeConfig({ scroll: { strategy: 'full_page', max_scroll_ms: 5000 } });
  assert.equal(config.scroll.strategy, 'full_page');
  assert.equal(config.scroll.max_scroll_ms, 5000);
  assert.equal(config.scroll.scroll_delay_ms, 250); // default preserved
});

test('mergeConfig ignores invalid scroll strategy', () => {
  const config = mergeConfig({ scroll: { strategy: 'invalid_strategy' } });
  assert.equal(config.scroll.strategy, 'viewport'); // default
});

test('mergeConfig ignores invalid priority', () => {
  const config = mergeConfig({ priority: 'urgent' });
  assert.equal(config.priority, 'normal'); // default
});

test('mergeConfig merges custom headers', () => {
  const config = mergeConfig({ headers: { 'Accept-Language': 'fr-CA' } });
  assert.equal(config.headers['Accept-Language'], 'fr-CA');
});

test('mergeConfig merges viewport', () => {
  const config = mergeConfig({ viewport: { width: 1920 } });
  assert.equal(config.viewport.width, 1920);
  assert.equal(config.viewport.height, 720); // default preserved
});

test('mergeConfig ignores non-string headers', () => {
  const config = mergeConfig({ headers: { valid: 'yes', invalid: 123 } });
  assert.equal(config.headers.valid, 'yes');
  assert.equal(config.headers.invalid, undefined);
});

test('mergeConfig ignores negative scroll values', () => {
  const config = mergeConfig({ scroll: { max_scroll_ms: -100, pixels: -50 } });
  assert.equal(config.scroll.max_scroll_ms, 10000); // default
  assert.equal(config.scroll.pixels, 0); // default
});

test('mergeConfig clamps percent to 0-100', () => {
  const config = mergeConfig({ scroll: { percent: 150 } });
  assert.equal(config.scroll.percent, 0); // rejected, default preserved
});

test('validateConfig passes for viewport strategy', () => {
  const config = mergeConfig({});
  assert.ok(validateConfig(config).valid);
});

test('validateConfig fails for pixels strategy with pixels=0', () => {
  const config = mergeConfig({ scroll: { strategy: 'pixels' } });
  const result = validateConfig(config);
  assert.equal(result.valid, false);
  assert.ok(result.error.includes('pixels'));
});

test('validateConfig fails for percent strategy with percent=0', () => {
  const config = mergeConfig({ scroll: { strategy: 'percent' } });
  const result = validateConfig(config);
  assert.equal(result.valid, false);
  assert.ok(result.error.includes('percent'));
});

test('validateConfig passes for percent strategy with valid percent', () => {
  const config = mergeConfig({ scroll: { strategy: 'percent', percent: 75 } });
  assert.ok(validateConfig(config).valid);
});

test('validateConfig passes for pixels strategy with valid pixels', () => {
  const config = mergeConfig({ scroll: { strategy: 'pixels', pixels: 500 } });
  assert.ok(validateConfig(config).valid);
});

test('SCROLL_STRATEGIES contains all four strategies', () => {
  assert.deepStrictEqual(SCROLL_STRATEGIES, ['viewport', 'full_page', 'percent', 'pixels']);
});

test('PRIORITY_LEVELS contains three levels', () => {
  assert.deepStrictEqual(PRIORITY_LEVELS, ['high', 'normal', 'low']);
});

// ─── Scroll Tests ───────────────────────────────────────────────────────────

test('scrollViewport returns zero-scroll result', async () => {
  const result = await scrollViewport(null, {});
  assert.equal(result.strategy_used, 'viewport');
  assert.equal(result.pixels_scrolled, 0);
  assert.equal(result.scroll_steps, 0);
  assert.equal(result.scroll_time_ms, 0);
});

test('STRATEGY_MAP has entries for all strategies', () => {
  assert.ok(STRATEGY_MAP.viewport);
  assert.ok(STRATEGY_MAP.full_page);
  assert.ok(STRATEGY_MAP.percent);
  assert.ok(STRATEGY_MAP.pixels);
  assert.equal(Object.keys(STRATEGY_MAP).length, 4);
});

// ─── PriorityQueue Tests ────────────────────────────────────────────────────

test('PriorityQueue starts with depth 0', () => {
  const q = new PriorityQueue();
  assert.equal(q.depth, 0);
  assert.deepStrictEqual(q.depthByPriority, { high: 0, normal: 0, low: 0 });
});

test('PriorityQueue enqueue and dequeue in priority order', () => {
  const q = new PriorityQueue();
  const responses = [];

  // Enqueue low, normal, high — dequeue should return high first
  q.enqueue({}, { url: 'low' }, { priority: 'low', source_id: null });
  q.enqueue({}, { url: 'normal' }, { priority: 'normal', source_id: null });
  q.enqueue({}, { url: 'high' }, { priority: 'high', source_id: null });

  assert.equal(q.depth, 3);

  const first = q.dequeue();
  assert.equal(first.body.url, 'high');

  const second = q.dequeue();
  assert.equal(second.body.url, 'normal');

  const third = q.dequeue();
  assert.equal(third.body.url, 'low');

  assert.equal(q.depth, 0);
});

test('PriorityQueue rejects when full', () => {
  const q = new PriorityQueue({ maxDepth: 2 });
  q.enqueue({}, {}, { priority: 'normal', source_id: null });
  q.enqueue({}, {}, { priority: 'normal', source_id: null });
  const result = q.enqueue({}, {}, { priority: 'normal', source_id: null });
  assert.equal(result.ok, false);
  assert.equal(result.error, 'queue full, try again later');
});

test('PriorityQueue respects per-source concurrency limit', () => {
  const q = new PriorityQueue({ maxConcurrentPerSource: 1, sourceCooldownMs: 0 });

  q.enqueue({}, { url: 'a1' }, { priority: 'normal', source_id: 'src-a' });
  q.enqueue({}, { url: 'a2' }, { priority: 'normal', source_id: 'src-a' });
  q.enqueue({}, { url: 'b1' }, { priority: 'normal', source_id: 'src-b' });

  // First dequeue: src-a (first item)
  const first = q.dequeue();
  assert.equal(first.body.url, 'a1');

  // Second dequeue: src-a is at limit (1), so skip a2, take b1
  const second = q.dequeue();
  assert.equal(second.body.url, 'b1');

  // Mark src-a complete — now a2 is eligible
  q.markComplete('src-a');
  const third = q.dequeue();
  assert.equal(third.body.url, 'a2');
});

test('PriorityQueue dequeue returns null when empty', () => {
  const q = new PriorityQueue();
  assert.equal(q.dequeue(), null);
});

test('PriorityQueue markComplete handles null source_id gracefully', () => {
  const q = new PriorityQueue();
  q.markComplete(null); // Should not throw
  assert.equal(q.activeBySource.size, 0);
});

test('PriorityQueue markComplete decrements active count', () => {
  const q = new PriorityQueue({ sourceCooldownMs: 0 });
  q.enqueue({}, {}, { priority: 'normal', source_id: 'src-a' });
  q.dequeue(); // Marks src-a active (count=1)
  assert.equal(q.activeBySource.get('src-a'), 1);
  q.markComplete('src-a');
  assert.equal(q.activeBySource.has('src-a'), false);
});

test('PriorityQueue expires stale items', () => {
  const q = new PriorityQueue({ maxQueueWaitMs: 1 }); // 1ms timeout

  // Create a mock response that captures the write
  let writtenStatus = null;
  const mockRes = {
    writeHead: (status) => { writtenStatus = status; },
    end: () => {},
  };

  q.enqueue(mockRes, { url: 'stale' }, { priority: 'normal', source_id: null });

  // Manually backdate the enqueued_at
  q.queues.normal[0].enqueued_at = Date.now() - 100;

  // Dequeue triggers expiry
  const result = q.dequeue();
  assert.equal(result, null); // Item was expired, not returned
  assert.equal(writtenStatus, 504);
  assert.equal(q.depth, 0);
});

test('PriorityQueue items without source_id are always eligible', () => {
  const q = new PriorityQueue({ maxConcurrentPerSource: 1, sourceCooldownMs: 0 });

  q.enqueue({}, { url: '1' }, { priority: 'normal', source_id: null });
  q.enqueue({}, { url: '2' }, { priority: 'normal', source_id: null });

  const first = q.dequeue();
  const second = q.dequeue();
  assert.equal(first.body.url, '1');
  assert.equal(second.body.url, '2');
});
