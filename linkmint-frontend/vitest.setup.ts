/**
 * Test setup: register jest-dom matchers, auto-clean the DOM between tests, and stub the browser
 * APIs jsdom lacks but Chakra's positioners (Dialog/Drawer/Menu/Tooltip), the responsive system, and
 * CopyField's clipboard copy all touch. Without these, kit components throw on render in jsdom.
 */

import { afterEach, vi } from 'vitest';
import { cleanup } from '@testing-library/react';
import { MotionGlobalConfig } from 'framer-motion';
import '@testing-library/jest-dom/vitest';

// Make framer-motion animations resolve instantly under jsdom (no rAF timing): motion components
// render their final state and `animate()` jumps to its target. Keeps motion assertions deterministic.
MotionGlobalConfig.skipAnimations = true;

afterEach(() => {
  cleanup();
});

// jsdom can't parse Chakra's `@layer`-based CSS and logs a (harmless) parse error per style insert.
// Swallow only that message so test output stays readable; every other console.error passes through.
const realConsoleError = console.error.bind(console);
console.error = (...args: unknown[]): void => {
  const blob = args
    .map((a) => (a instanceof Error ? `${a.message}\n${a.stack ?? ''}` : String(a)))
    .join(' ');
  if (blob.includes('Could not parse CSS stylesheet')) return;
  realConsoleError(...args);
};

// matchMedia — Chakra's responsive props + prefers-reduced-motion read this.
if (typeof window.matchMedia !== 'function') {
  window.matchMedia = (query: string): MediaQueryList =>
    ({
      matches: false,
      media: query,
      onchange: null,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
      addListener: () => undefined,
      removeListener: () => undefined,
      dispatchEvent: () => false,
    }) as unknown as MediaQueryList;
}

// ResizeObserver / IntersectionObserver — used by floating-ui positioners (popover/menu/tooltip).
class ObserverStub {
  observe(): void {
    /* no-op */
  }
  unobserve(): void {
    /* no-op */
  }
  disconnect(): void {
    /* no-op */
  }
  takeRecords(): [] {
    return [];
  }
}
globalThis.ResizeObserver ??= ObserverStub as unknown as typeof ResizeObserver;
globalThis.IntersectionObserver ??= ObserverStub as unknown as typeof IntersectionObserver;

// scrollIntoView — Ark UI scrolls highlighted menu/tabs items into view.
if (typeof Element.prototype.scrollIntoView !== 'function') {
  Element.prototype.scrollIntoView = (): void => undefined;
}

// Clipboard — CopyField / AddressChip call navigator.clipboard.writeText.
if (!navigator.clipboard) {
  Object.defineProperty(navigator, 'clipboard', {
    value: { writeText: vi.fn().mockResolvedValue(undefined) },
    configurable: true,
  });
}
