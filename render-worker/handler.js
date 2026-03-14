'use strict';

const { mergeConfig, validateConfig } = require('./config');

const MAX_BODY_BYTES = 1024 * 1024; // 1 MB request body limit
const MAX_QUEUE_DEPTH = 50; // reject with 503 when queue exceeds this

/**
 * Creates the HTTP request handler for the render worker.
 * @param {object} state - Shared mutable state (browser, requestCount, queueDepth, queue).
 * @param {Function} processQueue - Function to drain the render queue.
 * @returns {Function} Node.js http.Server request handler.
 */
function createRequestHandler(state, processQueue) {
  return function requestHandler(req, res) {
    if (req.method === 'GET' && req.url === '/health') {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({
        status: 'ok',
        browser_connected: state.browser ? state.browser.isConnected() : false,
        request_count: state.requestCount,
        queue_depth: state.queueDepth,
        recycle_after: state.recycleAfter,
      }));
      return;
    }

    if (req.method === 'POST' && req.url === '/render') {
      let data = '';
      let bodySize = 0;
      req.on('data', (chunk) => {
        bodySize += chunk.length;
        if (bodySize > MAX_BODY_BYTES) {
          res.writeHead(413, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({ error: 'request body too large' }));
          req.destroy();
          return;
        }
        data += chunk;
      });
      req.on('end', () => {
        if (bodySize > MAX_BODY_BYTES) return;
        try {
          const body = JSON.parse(data);
          if (!body.url) {
            res.writeHead(400, { 'Content-Type': 'application/json' });
            res.end(JSON.stringify({ error: 'url is required' }));
            return;
          }

          // M2: merge per-source config with defaults (backwards compatible — config is optional)
          const config = mergeConfig(body.config);
          const validation = validateConfig(config);
          if (!validation.valid) {
            res.writeHead(400, { 'Content-Type': 'application/json' });
            res.end(JSON.stringify({ error: validation.error }));
            return;
          }

          if (state.queue.length >= MAX_QUEUE_DEPTH) {
            res.writeHead(503, { 'Content-Type': 'application/json' });
            res.end(JSON.stringify({ error: 'queue full, try again later' }));
            return;
          }
          state.queue.push({ res, body, config });
          state.queueDepth = state.queue.length;
          processQueue();
        } catch (_err) {
          res.writeHead(400, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({ error: 'invalid JSON' }));
        }
      });
      return;
    }

    res.writeHead(404, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ error: 'not found' }));
  };
}

module.exports = { createRequestHandler, MAX_BODY_BYTES, MAX_QUEUE_DEPTH };
