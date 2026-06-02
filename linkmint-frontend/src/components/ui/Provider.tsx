'use client';

/**
 * Client-side providers for the app:
 * - Emotion SSR cache registry (via `useServerInsertedHTML`) so Chakra's styles are flushed into the
 *   document head during streaming — prevents the App-Router emotion hydration mismatch.
 * - Chakra UI (default system) and the Sonner toaster.
 * - Defensive cleanup of any stray service worker / CacheStorage left by a previous app at this
 *   origin (e.g. a Flutter SW at localhost:3000), which would otherwise intercept and break /v1 fetches.
 */

import { useEffect, useState, type ReactNode } from 'react';
import { useServerInsertedHTML } from 'next/navigation';
import createCache from '@emotion/cache';
import { CacheProvider } from '@emotion/react';
import { ChakraProvider } from '@chakra-ui/react';
import { Toaster } from 'sonner';
import { MotionConfig } from 'framer-motion';
import { system } from '@/theme/system';
import { ErrorBoundary } from '@/components/ErrorBoundary';
import { GlobalErrorOverlays } from '@/components/GlobalErrorOverlays';
import { OfflineBanner } from '@/components/OfflineBanner';
import { NotificationLiveRegion } from '@/components/notifications/NotificationLiveRegion';

export function Provider({ children }: { children: ReactNode }) {
  const [cache] = useState(() => {
    const emotionCache = createCache({ key: 'chakra' });
    emotionCache.compat = true;
    return emotionCache;
  });

  useServerInsertedHTML(() => (
    <style
      data-emotion={`${cache.key} ${Object.keys(cache.inserted).join(' ')}`}
      dangerouslySetInnerHTML={{ __html: Object.values(cache.inserted).join(' ') }}
    />
  ));

  useEffect(() => {
    if ('serviceWorker' in navigator) {
      void navigator.serviceWorker
        .getRegistrations()
        .then((regs) => regs.forEach((r) => void r.unregister()))
        .catch(() => {});
    }
    if ('caches' in window) {
      void caches
        .keys()
        .then((keys) => keys.forEach((k) => void caches.delete(k)))
        .catch(() => {});
    }
  }, []);

  return (
    <CacheProvider value={cache}>
      <ChakraProvider value={system}>
        {/* Global motion guard (ADR-012): every framer-motion component honours the OS
            prefers-reduced-motion setting; the CSS media query in globals.css is the backstop. */}
        <MotionConfig reducedMotion="user">
          {/* App-wide error system (work04): the boundary catches client render crashes; the overlays
              render the 401 re-auth / 402 KYC modals; the offline banner tracks connectivity. */}
          <OfflineBanner />
          <ErrorBoundary>{children}</ErrorBoundary>
          <GlobalErrorOverlays />
          {/* Announces new inbox arrivals to AT even when the panel is closed (work07 / F.6). */}
          <NotificationLiveRegion />
          {/* Governed toast layer (work07): richColors gives accessible per-kind palettes; closeButton
              makes every toast dismissible (F.6); the slide is reduced-motion-aware via the global
              prefers-reduced-motion backstop in globals.css (it zeroes transition-duration). All
              toasts flow through the typed `notify.*` wrapper (src/lib/notify.ts). */}
          <Toaster
            richColors
            closeButton
            position="top-right"
            duration={5000}
            gap={10}
            visibleToasts={4}
            toastOptions={{
              style: { fontFamily: "'Inter', system-ui, sans-serif", borderRadius: '10px' },
            }}
          />
        </MotionConfig>
      </ChakraProvider>
    </CacheProvider>
  );
}
