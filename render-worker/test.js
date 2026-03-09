'use strict';

const { test } = require('node:test');
const assert = require('node:assert/strict');
const http = require('node:http');
const { createRequestHandler, MAX_QUEUE_DEPTH } = require('./handler');

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
