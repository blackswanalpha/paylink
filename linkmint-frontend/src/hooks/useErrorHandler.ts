'use client';

/**
 * useErrorHandler — the declarative wrapper around `reportError` (work04). A component calls
 * `report(err)` in its catch path; when the resolved surface is `inline`, the hook stashes the
 * `DisplayError` in `inlineError` so the component can render an <ErrorBanner/> next to the action.
 * Toast / re-auth / KYC surfaces are dispatched by `reportError` and leave `inlineError` null.
 *
 * Pass defaults once (e.g. `useErrorHandler({ surface: 'inline' })` for a form); per-call `opts`
 * override them.
 */

import { useCallback, useState } from 'react';
import type { DisplayError } from '@/lib/errors';
import { reportError, type ReportOptions, type ReportResult } from '@/lib/reportError';

export type UseErrorHandlerOptions = ReportOptions;

export interface UseErrorHandlerApi {
  /** The most recent error when the resolved surface was `inline` (for rendering inline). */
  inlineError: DisplayError | null;
  /** Normalize → classify → route an error; merges per-call `opts` over the hook defaults. */
  report: (err: unknown, opts?: ReportOptions) => ReportResult;
  /** Clear the inline error (e.g. on a successful retry or a fresh submit). */
  clear: () => void;
}

export function useErrorHandler(defaults?: UseErrorHandlerOptions): UseErrorHandlerApi {
  const [inlineError, setInlineError] = useState<DisplayError | null>(null);

  // Destructure to primitives so `report` stays stable without depending on the (often inline) object.
  const defaultSurface = defaults?.surface;
  const defaultSilent = defaults?.silent;
  const defaultContext = defaults?.context;

  const report = useCallback(
    (err: unknown, opts?: ReportOptions): ReportResult => {
      const result = reportError(err, {
        surface: defaultSurface,
        silent: defaultSilent,
        context: defaultContext,
        ...opts,
      });
      if (result.surface === 'inline') {
        setInlineError(result.error);
      }
      return result;
    },
    [defaultSurface, defaultSilent, defaultContext],
  );

  const clear = useCallback(() => setInlineError(null), []);

  return { inlineError, report, clear };
}
