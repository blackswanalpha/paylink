/**
 * Normalize anything thrown by the SDK into a `DisplayError` the UI can render — surfacing the
 * standard LinkMint error envelope (code / message / details / trace id) and giving transport
 * failures actionable dev copy.
 */

import type { LinkMintApiError } from '@linkmint/sdk';
import {
  ConflictError,
  LinkMintConnectionError,
  LinkMintTimeoutError,
  RateLimitError,
  isLinkMintApiError,
} from '@linkmint/sdk';

export interface DisplayError {
  kind: 'api' | 'transport' | 'unknown';
  title: string;
  message: string;
  code?: string;
  status?: number;
  traceId?: string;
  requestId?: string;
  details?: Record<string, unknown>;
  retryAfter?: number;
}

function titleForApi(err: LinkMintApiError): string {
  if (err instanceof RateLimitError) {
    return 'Rate limited';
  }
  if (err instanceof ConflictError) {
    switch (err.code) {
      case 'PAYLINK_NOT_PAYABLE':
        return 'PayLink not payable yet';
      case 'PAYLINK_ALREADY_SETTLED':
        return 'Already settled';
      case 'PAYLINK_EXPIRED':
        return 'PayLink expired';
      default:
        return 'Conflict';
    }
  }
  switch (err.status) {
    case 400:
      return 'Invalid request';
    case 401:
      return 'Authentication failed';
    case 403:
      return 'Forbidden';
    case 404:
      return 'Not found';
    default:
      return err.status >= 500 ? 'Service error' : 'Request failed';
  }
}

/**
 * Build the message for a transport-level failure (a fetch `TypeError` surfaced as
 * `LinkMintConnectionError`). The error itself carries no cause, but a service worker controlling the
 * page is a strong signal the request was intercepted client-side — a stale Flutter SW left at this
 * origin is the usual culprit (see `Provider.tsx`), and that needs a different fix than a down stack.
 * Otherwise we steer toward the common case (a transient blip while the dev server restarts → reload)
 * before the heavier "is the stack up?" check.
 */
function transportError(): DisplayError {
  if (typeof navigator !== 'undefined' && navigator.serviceWorker?.controller) {
    return {
      kind: 'transport',
      title: 'Request blocked in the browser',
      message:
        'A service worker at this origin is intercepting requests — usually a stale one left by another app. ' +
        'Unregister it in DevTools (Application → Service Workers), clear site data, then hard-reload (Ctrl+Shift+R).',
    };
  }
  return {
    kind: 'transport',
    title: 'Cannot reach the API',
    message:
      'The request did not complete. If the dev server just restarted this is usually transient — reload the page. ' +
      'If it persists, check the local stack is up (docker compose up -d) and the gateway is reachable on :8088.',
  };
}

export function toDisplayError(err: unknown): DisplayError {
  if (isLinkMintApiError(err)) {
    return {
      kind: 'api',
      title: titleForApi(err),
      message: err.message,
      code: err.code,
      status: err.status,
      traceId: err.traceId,
      requestId: err.requestId,
      details: err.details,
      retryAfter: err instanceof RateLimitError ? err.retryAfter : undefined,
    };
  }
  if (err instanceof LinkMintTimeoutError) {
    return { kind: 'transport', title: 'Request timed out', message: err.message };
  }
  if (err instanceof LinkMintConnectionError) {
    return transportError();
  }
  if (err instanceof Error) {
    return { kind: 'unknown', title: 'Something went wrong', message: err.message };
  }
  return { kind: 'unknown', title: 'Something went wrong', message: String(err) };
}

/** True when the error is the SDK's caller-abort (used to ignore aborted polls on unmount). */
export function isAbortError(err: unknown): boolean {
  return err instanceof LinkMintConnectionError && /aborted/i.test(err.message);
}

// --- Error classification (work04) --------------------------------------------------------------
// The decision layer on top of `toDisplayError`: a pure mapping from a normalized `DisplayError` to
// *how* it should surface. Keeping this side-effect-free makes the whole policy unit-testable; the
// dispatch (toast / overlay / inline render) lives in `reportError` and the hooks.

/** Where an error should be shown. `reauth`/`kyc` are app-wide overlays; the rest are local. */
export type ErrorSurface = 'inline' | 'toast' | 'page' | 'reauth' | 'kyc';

export interface ErrorClassification {
  /** The default surface, derived purely from the envelope/kind. */
  surface: ErrorSurface;
  /** True only for 429 / 5xx / transport. Retry still requires the caller to supply a read fn. */
  canRetry: boolean;
  /** True when status forces the surface (401) — a caller hint cannot downgrade it. */
  forced: boolean;
  /** Seconds to wait before retry is allowed (429 `Retry-After`); undefined otherwise. */
  retryAfter?: number;
}

/**
 * Map a `DisplayError` to its presentation. Pure and total — every kind/status lands somewhere.
 *
 * Rationale baked in: 409 is a *mutation* collision, so it is never retryable even though it's a 4xx
 * (the idempotent-reads-only rule); 401 is `forced` because an expired session must interrupt the
 * whole app; 402 (`KYC_REQUIRED`) defaults to the global KYC overlay but is *not* forced, so a screen
 * can opt into an inline gate via `reportError(err, { surface: 'inline' })`.
 */
export function classifyError(e: DisplayError): ErrorClassification {
  if (e.kind === 'transport') {
    return { surface: 'toast', canRetry: true, forced: false };
  }
  if (e.kind === 'unknown') {
    return { surface: 'toast', canRetry: false, forced: false };
  }
  switch (e.status) {
    case 401:
      return { surface: 'reauth', canRetry: false, forced: true };
    case 402:
      return { surface: 'kyc', canRetry: false, forced: false };
    case 429:
      return { surface: 'inline', canRetry: true, forced: false, retryAfter: e.retryAfter };
    case 400:
    case 403:
    case 404:
    case 409:
      return { surface: 'inline', canRetry: false, forced: false };
    default:
      if (typeof e.status === 'number' && e.status >= 500) {
        return { surface: 'toast', canRetry: true, forced: false };
      }
      return { surface: 'inline', canRetry: false, forced: false };
  }
}

/**
 * A short, human-shareable id for a *render crash* — those have no envelope, so there is no
 * `trace_id` to surface. Support correlates by this id + timestamp. Browser-only; falls back to
 * `Math.random` where `crypto.randomUUID` is unavailable.
 */
export function newErrorId(): string {
  try {
    if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
      return crypto.randomUUID().slice(0, 8).toUpperCase();
    }
  } catch {
    // fall through to the Math.random path
  }
  return Math.random().toString(36).slice(2, 10).toUpperCase();
}
