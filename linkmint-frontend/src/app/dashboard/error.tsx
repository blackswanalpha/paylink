'use client';

/**
 * Dashboard segment error boundary (work04) — demonstrates per-route error handling: a crash inside
 * the dashboard segment renders this scoped fallback (within the root layout, so the shell/theme stay
 * intact) instead of taking down the whole app. Mirrors the root `error.tsx`.
 */

import { useMemo } from 'react';
import { newErrorId } from '@/lib/errors';
import { ErrorFallback } from '@/components/ErrorFallback';

export default function DashboardError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  const id = useMemo(() => error.digest ?? newErrorId(), [error]);

  return (
    <ErrorFallback
      id={id}
      title="This section hit an error"
      description="The dashboard couldn’t render. Try again, or head back home."
      onReset={reset}
      showHome
    />
  );
}
