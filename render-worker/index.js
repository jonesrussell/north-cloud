'use strict';

const http = require('http');
const { chromium } = require('playwright');
const { createRequestHandler } = require('./handler');
const { STEALTH_INIT_SCRIPT } = require('./stealth');

const PORT = process.env.PORT || 3000;
const RECYCLE_AFTER = parseInt(process.env.BROWSER_RECYCLE_AFTER || '100', 10);
const DEFAULT_TIMEOUT = parseInt(process.env.DEFAULT_TIMEOUT_MS || '15000', 10);

const state = {
  browser: null,
  requestCount: 0,
  queueDepth: 0,
  queue: [],
  recycleAfter: RECYCLE_AFTER,
};

let processing = false;

async function ensureBrowser() {
  if (!state.browser || !state.browser.isConnected()) {
    state.browser = await chromium.launch({
      args: [
        '--no-sandbox',
        '--disable-setuid-sandbox',
        '--disable-dev-shm-usage',
        '--disable-blink-features=AutomationControlled',
      ],
    });
    state.requestCount = 0;
  }
  return state.browser;
}

async function recycleBrowserIfNeeded() {
  if (state.requestCount >= RECYCLE_AFTER && state.browser) {
    await state.browser.close().catch(() => {});
    state.browser = null;
  }
}

async function renderPage(url, timeoutMs, waitUntil) {
  const b = await ensureBrowser();
  const context = await b.newContext({
    userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36',
  });
  const page = await context.newPage();
  await page.addInitScript(STEALTH_INIT_SCRIPT);

  try {
    const start = Date.now();
    const response = await page.goto(url, {
      timeout: timeoutMs,
      waitUntil: waitUntil || 'networkidle',
    });

    const html = await page.content();
    const finalUrl = page.url();
    const renderTimeMs = Date.now() - start;
    const statusCode = response ? response.status() : 0;

    return { html, final_url: finalUrl, render_time_ms: renderTimeMs, status_code: statusCode };
  } finally {
    await page.close().catch(() => {});
    await context.close().catch(() => {});
    state.requestCount++;
    await recycleBrowserIfNeeded();
  }
}

function processQueue() {
  if (processing || state.queue.length === 0) return;
  processing = true;
  const { res, body } = state.queue.shift();
  state.queueDepth = state.queue.length;

  renderPage(body.url, body.timeout_ms || DEFAULT_TIMEOUT, body.wait_until)
    .then((result) => {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(result));
    })
    .catch((err) => {
      res.writeHead(500, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ error: err.message }));
    })
    .finally(() => {
      processing = false;
      processQueue();
    });
}

const server = http.createServer(createRequestHandler(state, processQueue));

server.listen(PORT, () => {
  console.log(`render-worker listening on port ${PORT}`);
  ensureBrowser().then(() => console.log('browser launched'));
});

process.on('SIGTERM', async () => {
  console.log('shutting down');
  if (state.browser) await state.browser.close().catch(() => {});
  server.close();
  process.exit(0);
});
