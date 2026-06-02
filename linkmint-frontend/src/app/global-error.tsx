'use client';

/**
 * Root-layout error fallback (work04) — catches errors thrown in the root layout itself. It REPLACES
 * the whole document, so it must render its own <html>/<body> and cannot rely on the Chakra provider
 * (which may be the very thing that failed). Hence inline brand styling: the one sanctioned exception
 * to the "semantic tokens only" rule — canvas #FAF7F0 / ink #1C1A17 / emerald #0F6E4E from the theme.
 */

import { useEffect, useMemo } from 'react';
import { newErrorId } from '@/lib/errors';

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  const id = useMemo(() => error.digest ?? newErrorId(), [error]);

  useEffect(() => {
    console.error('Global error:', error);
  }, [error]);

  return (
    <html lang="en">
      <body
        style={{
          margin: 0,
          background: '#FAF7F0',
          color: '#1C1A17',
          fontFamily: "'Inter', system-ui, sans-serif",
        }}
      >
        <main
          style={{ minHeight: '100vh', display: 'grid', placeItems: 'center', padding: '2rem' }}
        >
          <div
            style={{ maxWidth: '28rem', textAlign: 'center' }}
            role="alert"
            aria-live="assertive"
          >
            <h1
              style={{
                fontFamily: "'Fraunces', Georgia, serif",
                fontSize: '1.5rem',
                margin: '0 0 0.5rem',
              }}
            >
              Something went wrong
            </h1>
            <p style={{ color: '#6B655C', margin: '0 0 1.5rem' }}>
              A critical error interrupted the app. Reloading usually fixes it.
            </p>
            <button
              type="button"
              onClick={reset}
              style={{
                background: '#0F6E4E',
                color: '#FFFFFF',
                border: 'none',
                borderRadius: '10px',
                padding: '0.625rem 1.25rem',
                fontSize: '0.95rem',
                cursor: 'pointer',
              }}
            >
              Reload
            </button>
            <p
              style={{
                color: '#8A7E6A',
                fontSize: '0.75rem',
                marginTop: '1.5rem',
                fontFamily: "'JetBrains Mono', monospace",
              }}
            >
              error id: {id}
            </p>
          </div>
        </main>
      </body>
    </html>
  );
}
