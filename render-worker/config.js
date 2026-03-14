'use strict';

/**
 * Per-source render configuration schema and defaults.
 * M2 extension — all fields are optional for backwards compatibility with M1.
 */

const SCROLL_STRATEGIES = ['viewport', 'full_page', 'percent', 'pixels'];
const PRIORITY_LEVELS = ['high', 'normal', 'low'];

const DEFAULT_CONFIG = {
  scroll: {
    strategy: 'viewport',
    max_scroll_ms: 10000,
    scroll_delay_ms: 250,
    pixels: 0,
    percent: 0,
  },
  selectors: {
    wait_for: null,
    wait_timeout_ms: 5000,
    extract: null,
  },
  headers: {},
  viewport: {
    width: 1280,
    height: 720,
  },
  priority: 'normal',
  source_id: null,
};

/**
 * Deep-merges user config onto defaults, producing a complete config object.
 * Only known fields are merged — unknown fields are silently dropped.
 * @param {object|undefined} userConfig - Partial config from the request body.
 * @returns {object} Complete config with all defaults applied.
 */
function mergeConfig(userConfig) {
  if (!userConfig || typeof userConfig !== 'object') {
    return structuredClone(DEFAULT_CONFIG);
  }

  const merged = structuredClone(DEFAULT_CONFIG);

  // Scroll config
  if (userConfig.scroll && typeof userConfig.scroll === 'object') {
    if (SCROLL_STRATEGIES.includes(userConfig.scroll.strategy)) {
      merged.scroll.strategy = userConfig.scroll.strategy;
    }
    if (typeof userConfig.scroll.max_scroll_ms === 'number' && userConfig.scroll.max_scroll_ms > 0) {
      merged.scroll.max_scroll_ms = userConfig.scroll.max_scroll_ms;
    }
    if (typeof userConfig.scroll.scroll_delay_ms === 'number' && userConfig.scroll.scroll_delay_ms > 0) {
      merged.scroll.scroll_delay_ms = userConfig.scroll.scroll_delay_ms;
    }
    if (typeof userConfig.scroll.pixels === 'number' && userConfig.scroll.pixels >= 0) {
      merged.scroll.pixels = userConfig.scroll.pixels;
    }
    if (typeof userConfig.scroll.percent === 'number' && userConfig.scroll.percent >= 0 && userConfig.scroll.percent <= 100) {
      merged.scroll.percent = userConfig.scroll.percent;
    }
  }

  // Selector config
  if (userConfig.selectors && typeof userConfig.selectors === 'object') {
    if (typeof userConfig.selectors.wait_for === 'string') {
      merged.selectors.wait_for = userConfig.selectors.wait_for;
    }
    if (typeof userConfig.selectors.wait_timeout_ms === 'number' && userConfig.selectors.wait_timeout_ms > 0) {
      merged.selectors.wait_timeout_ms = userConfig.selectors.wait_timeout_ms;
    }
    if (typeof userConfig.selectors.extract === 'string') {
      merged.selectors.extract = userConfig.selectors.extract;
    }
  }

  // Custom headers
  if (userConfig.headers && typeof userConfig.headers === 'object') {
    for (const [key, value] of Object.entries(userConfig.headers)) {
      if (typeof key === 'string' && typeof value === 'string') {
        merged.headers[key] = value;
      }
    }
  }

  // Viewport
  if (userConfig.viewport && typeof userConfig.viewport === 'object') {
    if (typeof userConfig.viewport.width === 'number' && userConfig.viewport.width > 0) {
      merged.viewport.width = userConfig.viewport.width;
    }
    if (typeof userConfig.viewport.height === 'number' && userConfig.viewport.height > 0) {
      merged.viewport.height = userConfig.viewport.height;
    }
  }

  // Priority
  if (PRIORITY_LEVELS.includes(userConfig.priority)) {
    merged.priority = userConfig.priority;
  }

  // Source ID
  if (typeof userConfig.source_id === 'string' && userConfig.source_id.length > 0) {
    merged.source_id = userConfig.source_id;
  }

  return merged;
}

/**
 * Validates that a scroll config is internally consistent.
 * @param {object} config - Merged config object.
 * @returns {{ valid: boolean, error?: string }}
 */
function validateConfig(config) {
  const { strategy, pixels, percent } = config.scroll;

  if (strategy === 'pixels' && pixels <= 0) {
    return { valid: false, error: 'scroll.pixels must be > 0 when strategy is "pixels"' };
  }

  if (strategy === 'percent' && (percent <= 0 || percent > 100)) {
    return { valid: false, error: 'scroll.percent must be between 1 and 100 when strategy is "percent"' };
  }

  return { valid: true };
}

module.exports = {
  SCROLL_STRATEGIES,
  PRIORITY_LEVELS,
  DEFAULT_CONFIG,
  mergeConfig,
  validateConfig,
};
