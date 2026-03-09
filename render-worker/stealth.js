'use strict';

/**
 * Stealth init script injected into every Playwright page before navigation.
 * Patches browser fingerprints that headless-detection systems check to avoid 403s:
 *   - navigator.webdriver  → undefined   (automation flag)
 *   - navigator.plugins    → non-empty   (headless has 0 plugins)
 *   - navigator.languages  → ['en-US','en'] (headless is often empty)
 */
const STEALTH_INIT_SCRIPT = `
  Object.defineProperty(navigator, 'webdriver', { get: () => undefined });
  Object.defineProperty(navigator, 'plugins', {
    get: () => {
      const plugins = [
        { name: 'Chrome PDF Plugin', filename: 'internal-pdf-viewer', description: 'Portable Document Format' },
        { name: 'Chrome PDF Viewer', filename: 'mhjfbmdgcfjbbpaeojofohoefgiehjai', description: '' },
        { name: 'Native Client', filename: 'internal-nacl-plugin', description: '' },
      ];
      plugins.refresh = () => {};
      plugins.item = (i) => plugins[i] || null;
      plugins.namedItem = (n) => plugins.find((p) => p.name === n) || null;
      return plugins;
    },
  });
  Object.defineProperty(navigator, 'languages', { get: () => ['en-US', 'en'] });
`;

module.exports = { STEALTH_INIT_SCRIPT };
