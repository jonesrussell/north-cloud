'use strict';

/**
 * Scroll strategy implementations for the render worker.
 *
 * Each strategy function receives a Playwright page and scroll config,
 * performs the scroll action, and returns metadata about what was done.
 *
 * M2 scaffolding — these functions will be wired into renderPage() in a follow-up PR.
 */

/**
 * @typedef {object} ScrollResult
 * @property {string} strategy_used - The scroll strategy that was executed.
 * @property {number} pixels_scrolled - Total pixels scrolled from top.
 * @property {number} scroll_steps - Number of discrete scroll actions.
 * @property {number} scroll_time_ms - Wall time spent scrolling.
 */

/**
 * No-op scroll — returns immediately. Used for static pages (M1 behavior).
 * @param {import('playwright').Page} _page
 * @param {object} _scrollConfig
 * @returns {Promise<ScrollResult>}
 */
async function scrollViewport(_page, _scrollConfig) {
  return {
    strategy_used: 'viewport',
    pixels_scrolled: 0,
    scroll_steps: 0,
    scroll_time_ms: 0,
  };
}

/**
 * Auto-scrolls to the bottom of the page in viewport-height increments.
 * Stops when page height stabilizes or max_scroll_ms is exceeded.
 * @param {import('playwright').Page} page
 * @param {object} scrollConfig
 * @returns {Promise<ScrollResult>}
 */
async function scrollFullPage(page, scrollConfig) {
  const { max_scroll_ms, scroll_delay_ms } = scrollConfig;
  const start = Date.now();
  let steps = 0;
  let stableCount = 0;
  let lastHeight = 0;

  // eslint-disable-next-line no-constant-condition
  while (true) {
    const elapsed = Date.now() - start;
    if (elapsed >= max_scroll_ms) break;

    const { scrollHeight, innerHeight } = await page.evaluate(() => ({
      scrollHeight: document.documentElement.scrollHeight,
      innerHeight: window.innerHeight,
    }));

    if (scrollHeight === lastHeight) {
      stableCount++;
      if (stableCount >= 2) break; // Height unchanged after 2 consecutive scrolls
    } else {
      stableCount = 0;
      lastHeight = scrollHeight;
    }

    await page.evaluate((vh) => window.scrollBy(0, vh), innerHeight);
    steps++;

    await page.waitForTimeout(scroll_delay_ms);
  }

  const finalScroll = await page.evaluate(() => window.scrollY);

  return {
    strategy_used: 'full_page',
    pixels_scrolled: finalScroll,
    scroll_steps: steps,
    scroll_time_ms: Date.now() - start,
  };
}

/**
 * Scrolls to a target percentage of total page height.
 * @param {import('playwright').Page} page
 * @param {object} scrollConfig
 * @returns {Promise<ScrollResult>}
 */
async function scrollPercent(page, scrollConfig) {
  const { percent, scroll_delay_ms } = scrollConfig;
  const start = Date.now();

  const scrollHeight = await page.evaluate(() => document.documentElement.scrollHeight);
  const targetY = Math.round(scrollHeight * (percent / 100));

  await page.evaluate((y) => window.scrollTo(0, y), targetY);
  await page.waitForTimeout(scroll_delay_ms);

  const finalScroll = await page.evaluate(() => window.scrollY);

  return {
    strategy_used: 'percent',
    pixels_scrolled: finalScroll,
    scroll_steps: 1,
    scroll_time_ms: Date.now() - start,
  };
}

/**
 * Scrolls to an exact pixel offset.
 * @param {import('playwright').Page} page
 * @param {object} scrollConfig
 * @returns {Promise<ScrollResult>}
 */
async function scrollPixels(page, scrollConfig) {
  const { pixels, scroll_delay_ms } = scrollConfig;
  const start = Date.now();

  await page.evaluate((y) => window.scrollTo(0, y), pixels);
  await page.waitForTimeout(scroll_delay_ms);

  const finalScroll = await page.evaluate(() => window.scrollY);

  return {
    strategy_used: 'pixels',
    pixels_scrolled: finalScroll,
    scroll_steps: 1,
    scroll_time_ms: Date.now() - start,
  };
}

/** Map of strategy name → implementation function. */
const STRATEGY_MAP = {
  viewport: scrollViewport,
  full_page: scrollFullPage,
  percent: scrollPercent,
  pixels: scrollPixels,
};

/**
 * Executes the appropriate scroll strategy for the given config.
 * @param {import('playwright').Page} page - Playwright page instance.
 * @param {object} scrollConfig - The scroll portion of the render config.
 * @returns {Promise<ScrollResult>}
 */
async function executeScroll(page, scrollConfig) {
  const fn = STRATEGY_MAP[scrollConfig.strategy];
  if (!fn) {
    return scrollViewport(page, scrollConfig); // Fallback to no-op
  }
  return fn(page, scrollConfig);
}

module.exports = {
  scrollViewport,
  scrollFullPage,
  scrollPercent,
  scrollPixels,
  executeScroll,
  STRATEGY_MAP,
};
