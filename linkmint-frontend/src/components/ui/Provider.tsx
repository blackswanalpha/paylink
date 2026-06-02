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
import { system } from '@/theme/system';
import { ErrorBoundary } from '@/components/ErrorBoundary';
import { GlobalErrorOverlays } from '@/components/GlobalErrorOverlays';
import { OfflineBanner } from '@/components/OfflineBanner';

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
        {/* App-wide error system (work04): the boundary catches client render crashes; the overlays
            render the 401 re-auth / 402 KYC modals; the offline banner tracks connectivity. */}
        <OfflineBanner />
        <ErrorBoundary>{children}</ErrorBoundary>
        <GlobalErrorOverlays />
        <Toaster
          richColors
          closeButton
          position="top-right"
          toastOptions={{ style: { fontFamily: "'Inter', system-ui, sans-serif" } }}
        />
      </ChakraProvider>
    </CacheProvider>
  );
}
