/**
 * reportError — the one imperative entry point feature code calls in a `catch` block (work04 / F.5).
 * It normalizes the thrown value (SDK error hierarchy only — F.1), classifies it, then *routes* it:
 *
 *   reauth / kyc  → the app-wide overlay store (`useErrorStore`)
 *   toast         → Sonner (the transport already mounted in Provider; the richer center is work07)
 *   inline / page → no side effect — the caller renders it (e.g. <ErrorBanner/>), and we return the
 *                   normalized `DisplayError` so they can
 *
 * It's a plain function (no hooks) so both React components and non-React code (the class
 * ErrorBoundary) can call it. The hook wrapper `useErrorHandler` builds on top of this.
 */

import { toast } from 'sonner';
import {
  classifyError,
  toDisplayError,
  type DisplayError,
  type ErrorClassification,
  type ErrorSurface,
} from '@/lib/errors';
import { useErrorStore } from '@/store/errors';

export interface ReportOptions {
  /**
   * Force the surface — but only a *downgrade* among `inline | toast | page`. A `forced`
   * classification (401 re-auth) ignores this; 402 can be steered to `inline` for a contextual gate.
   */
  surface?: 'inline' | 'toast' | 'page';
  /** Do nothing visible; just normalize + classify and return it (the caller renders). */
  silent?: boolean;
  /** Extra context appended to a toast description, e.g. "while creating a PayLink". */
  context?: string;
}

export interface ReportResult {
  error: DisplayError;
  classification: ErrorClassification;
  /** The surface actually used after applying `opts` + the `forced` rule. */
  surface: ErrorSurface;
}

/** Resolve the effective surface: a forced classification wins; otherwise the caller hint wins. */
function resolveSurface(classification: ErrorClassification, opts?: ReportOptions): ErrorSurface {
  if (classification.forced) {
    return classification.surface;
  }
  return opts?.surface ?? classification.surface;
}

export function reportError(err: unknown, opts?: ReportOptions): ReportResult {
  const error = toDisplayError(err);
  const classification = classifyError(error);
  const surface = resolveSurface(classification, opts);

  if (opts?.silent) {
    return { error, classification, surface };
  }

  switch (surface) {
    case 'reauth':
      useErrorStore.getState().requireReauth(error);
      break;
    case 'kyc':
      useErrorStore.getState().requireKyc(error);
      break;
    case 'toast': {
      const description = [error.message, opts?.context].filter(Boolean).join(' — ');
      toast.error(error.title, { description });
      break;
    }
    case 'inline':
    case 'page':
      // Rendered by the caller (inline banner / route fallback) — nothing to dispatch here.
      break;
  }

  return { error, classification, surface };
}
