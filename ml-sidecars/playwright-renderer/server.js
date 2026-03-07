'use strict';

const express = require('express');
const { chromium } = require('playwright');

const PORT = parseInt(process.env.RENDERER_PORT || '8095', 10);
const DEFAULT_TIMEOUT_MS = parseInt(process.env.RENDERER_TIMEOUT_MS || '30000', 10);
const MAX_CONCURRENT = parseInt(process.env.RENDERER_MAX_CONCURRENT || '3', 10);

let browser = null;
let activeRequests = 0;

async function getBrowser() {
  if (!browser || !browser.isConnected()) {
    browser = await chromium.launch({
      headless: true,
      args: ['--no-sandbox', '--disable-setuid-sandbox', '--disable-dev-shm-usage'],
    });
  }
  return browser;
}

const app = express();
app.use(express.json());

// Health check
app.get('/health', (_req, res) => {
  res.json({ status: 'ok', active_requests: activeRequests });
});

// Render endpoint
app.post('/render', async (req, res) => {
  const { url, wait_for = 'networkidle', timeout_ms = DEFAULT_TIMEOUT_MS } = req.body || {};

  if (!url) {
    return res.status(400).json({ error: 'url is required' });
  }

  if (activeRequests >= MAX_CONCURRENT) {
    return res.status(503).json({ error: 'renderer busy, try again shortly' });
  }

  activeRequests++;
  let page = null;

  try {
    const b = await getBrowser();
    page = await b.newPage();

    // Block unnecessary resource types to speed up rendering
    await page.route('**/*', (route) => {
      const type = route.request().resourceType();
      if (['image', 'media', 'font', 'stylesheet'].includes(type)) {
        return route.abort();
      }
      return route.continue();
    });

    await page.goto(url, {
      waitUntil: wait_for === 'domcontentloaded' ? 'domcontentloaded' : 'networkidle',
      timeout: Math.min(timeout_ms, 60000),
    });

    const html = await page.content();
    res.json({ html, url });
  } catch (err) {
    res.status(500).json({ error: err.message });
  } finally {
    if (page) {
      await page.close().catch(() => {});
    }
    activeRequests--;
  }
});

async function main() {
  // Pre-warm browser
  await getBrowser();
  console.error(`Playwright renderer ready on port ${PORT}`);

  app.listen(PORT, () => {
    console.error(`Listening on :${PORT}`);
  });
}

main().catch((err) => {
  console.error('Fatal:', err);
  process.exit(1);
});
