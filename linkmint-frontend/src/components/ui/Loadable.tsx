'use client';

/**
 * Loadable / AsyncBoundary (work06 / frontendfeature.md §1) — the loading/empty/data sequencer for
 * hook-fetched data (the app fetches via useState/useEffect hooks, not React Suspense).
 *
 * `Loadable` encodes the precedence (and the initial-vs-refresh rule — the dashboard pattern):
 *   1. error      → render nothing here; the caller owns the error path (F.5: an empty-because-it-
 *                   errored must NOT show a fake/branded empty state). On a *failed refresh* (data
 *                   already on screen) the stale data is kept so the page doesn't blank out.
 *   2. initial    → loading && no data yet → render `skeleton`.
 *   3. empty      → isEmpty && not loading → render `empty` (a branded catalog empty).
 *   4. data       → render `children` (also the refresh case: loading && data present, so no skeleton
 *                   flash — F.7).
 *
 * `Loadable` deliberately does not render the error itself: work04 owns error UI and `reportError`'s
 * contract is "the caller renders inline/page errors" (see lib/reportError.ts). Screens already render
 * `<ErrorBanner error={error} onRetry={refresh}/>`; `Loadable` only *suppresses* empty/skeleton when
 * errored so F.5 holds, staying decoupled from `ErrorBanner`.
 *
 * `AsyncBoundary` is the boundary form (Suspense + the existing work04 `ErrorBoundary`) for render-time
 * errors / future Suspense data sources.
 */

import { Suspense, type ReactNode } from 'react';
import { ErrorBoundary } from '../ErrorBoundary';

export interface LoadableProps {
  /** True while a fetch is in flight (initial OR refresh). */
  loading: boolean;
  /**
   * Truthy when the most recent fetch errored (pass the `DisplayError | null`). When set, `Loadable`
   * renders nothing on initial load so the caller's error path owns the surface — never a fake empty
   * (F.5). On a failed refresh (data already present) the stale data is kept.
   */
  error?: unknown;
  /** True when the fetch succeeded but there is no data to show. */
  isEmpty: boolean;
  /** Whether any data has loaded at least once (drives initial-vs-refresh). @default `!isEmpty` */
  hasData?: boolean;
  /** The skeleton composition for the initial load (an aria-busy region from skeletons.tsx). */
  skeleton: ReactNode;
  /** The branded empty state (a catalog wrapper from emptyStates.tsx). */
  empty: ReactNode;
  /** The real, loaded content. */
  children: ReactNode;
}

export function Loadable({
  loading,
  error,
  isEmpty,
  hasData,
  skeleton,
  empty,
  children,
}: LoadableProps) {
  const dataPresent = hasData ?? !isEmpty;

  // F.5 — an error never renders a (fake) empty/skeleton here; the caller renders the error path.
  if (error) {
    return dataPresent ? <>{children}</> : null;
  }
  // Initial load only: skeleton when loading AND nothing has loaded yet (no skeleton on refresh, F.7).
  if (loading && !dataPresent) {
    return <>{skeleton}</>;
  }
  if (isEmpty) {
    return <>{empty}</>;
  }
  return <>{children}</>;
}

export interface AsyncBoundaryProps {
  children: ReactNode;
  /** Suspense fallback — pass a skeleton composition. */
  fallback: ReactNode;
  /** Optional custom error fallback; defaults to ErrorBoundary's branded ErrorFallback. */
  errorFallback?: (props: { id: string; reset: () => void }) => ReactNode;
}

/** Suspense + the work04 `ErrorBoundary`, for render-time errors / future Suspense data. */
export function AsyncBoundary({ children, fallback, errorFallback }: AsyncBoundaryProps) {
  return (
    <ErrorBoundary fallback={errorFallback}>
      <Suspense fallback={fallback}>{children}</Suspense>
    </ErrorBoundary>
  );
}
