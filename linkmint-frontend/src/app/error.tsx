'use client';

/**
 * Route-segment error fallback (work04) — Next's App Router boundary for render errors thrown below
 * the root layout. Renders the shared branded `ErrorFallback` with the error's `digest` (when the
 * server provides one) or a generated id, plus Next's `reset` to retry the segment.
 */

import { useEffect, useMemo } from 'react';
import { newErrorId } from '@/lib/errors';
import { ErrorFallback } from '@/components/ErrorFallback';

export default function Error({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  const id = useMemo(() => error.digest ?? newErrorId(), [error]);

  useEffect(() => {
    console.error('Route error:', error);
  }, [error]);

  return <ErrorFallback id={id} onReset={reset} showHome />;
}
