const http = require('http');
const { chromium } = require('playwright');

const PORT = process.env.PORT || 3000;
const MAX_CONCURRENT = parseInt(process.env.MAX_CONCURRENT_TABS || '1', 10);
const RECYCLE_AFTER = parseInt(process.env.BROWSER_RECYCLE_AFTER || '100', 10);
const DEFAULT_TIMEOUT = parseInt(process.env.DEFAULT_TIMEOUT_MS || '15000', 10);
const MAX_BODY_BYTES = 1024 * 1024; // 1 MB request body limit
const MAX_QUEUE_DEPTH = 50; // reject with 503 when queue exceeds this

let browser = null;
let requestCount = 0;
let queueDepth = 0;
let processing = false;
const queue = [];

async function ensureBrowser() {
  if (!browser || !browser.isConnected()) {
    browser = await chromium.launch({
      args: ['--no-sandbox', '--disable-setuid-sandbox', '--disable-dev-shm-usage'],
    });
    requestCount = 0;
  }
  return browser;
}

async function recycleBrowserIfNeeded() {
  if (requestCount >= RECYCLE_AFTER && browser) {
    await browser.close().catch(() => {});
    browser = null;
  }
}

async function renderPage(url, timeoutMs, waitUntil) {
  const b = await ensureBrowser();
  const context = await b.newContext({
    userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36',
  });
  const page = await context.newPage();

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
    requestCount++;
    await recycleBrowserIfNeeded();
  }
}

function processQueue() {
  if (processing || queue.length === 0) return;
  processing = true;
  const { req, res, body } = queue.shift();
  queueDepth = queue.length;

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

const server = http.createServer((req, res) => {
  if (req.method === 'GET' && req.url === '/health') {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({
      status: 'ok',
      browser_connected: browser ? browser.isConnected() : false,
      request_count: requestCount,
      queue_depth: queueDepth,
      recycle_after: RECYCLE_AFTER,
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
        if (queue.length >= MAX_QUEUE_DEPTH) {
          res.writeHead(503, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({ error: 'queue full, try again later' }));
          return;
        }
        queue.push({ req, res, body });
        queueDepth = queue.length;
        processQueue();
      } catch (err) {
        res.writeHead(400, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ error: 'invalid JSON' }));
      }
    });
    return;
  }

  res.writeHead(404, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify({ error: 'not found' }));
});

server.listen(PORT, () => {
  console.log(`render-worker listening on port ${PORT}`);
  ensureBrowser().then(() => console.log('browser launched'));
});

process.on('SIGTERM', async () => {
  console.log('shutting down');
  if (browser) await browser.close().catch(() => {});
  server.close();
  process.exit(0);
});
