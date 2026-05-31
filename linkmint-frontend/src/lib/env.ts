/**
 * Client-readable configuration, sourced from `NEXT_PUBLIC_*` env vars (inlined by Next at build).
 * Server-only secrets (the JWT signing secret) are NOT read here — see `lib/jwt.ts`.
 */

export interface ClientConfig {
  /** Presentational MPesa Pay Bill / Till number for the instructions view. */
  mpesaPaybill: string;
  /** Settlement poll interval in milliseconds. */
  pollMs: number;
  /** Default currency pre-filled in the create form. */
  defaultCurrency: string;
  /** Default receiver address pre-filled in the create form (may be empty). */
  defaultReceiver: string;
}

function positiveInt(raw: string | undefined, fallback: number): number {
  const parsed = Number.parseInt(raw ?? '', 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
}

export function clientConfig(): ClientConfig {
  return {
    mpesaPaybill: process.env.NEXT_PUBLIC_MPESA_PAYBILL || '174379',
    pollMs: positiveInt(process.env.NEXT_PUBLIC_SETTLEMENT_POLL_MS, 2500),
    defaultCurrency: process.env.NEXT_PUBLIC_DEFAULT_CURRENCY || 'KES',
    defaultReceiver: process.env.NEXT_PUBLIC_DEFAULT_RECEIVER || '',
  };
}

/**
 * Resolve the SDK `baseUrl`: an explicit override if set, otherwise the app's own origin so that
 * `/v1/*` stays same-origin and is proxied to the gateway by the Next rewrite. During SSR (no
 * `window`) a valid placeholder is returned — real API calls only happen client-side.
 */
export function resolveApiBaseUrl(): string {
  const override = process.env.NEXT_PUBLIC_LINKMINT_BASE_URL;
  if (override && override.length > 0) {
    return override;
  }
  if (typeof window !== 'undefined') {
    return window.location.origin;
  }
  return 'http://localhost:3000';
}
