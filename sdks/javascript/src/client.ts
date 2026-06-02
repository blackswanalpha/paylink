/** The top-level LinkMint client: composes the `/v1` resources over a configured transport. */

import { LinkMintError } from './errors';
import { HttpClient, type AuthConfig, type FetchLike, type ResolvedConfig } from './http';
import { defaultIdempotencyKey } from './idempotency';
import { NotificationsResource } from './resources/notifications';
import { PayLinksResource } from './resources/paylinks';
import { PaymentsResource } from './resources/payments';

const DEFAULT_TIMEOUT_MS = 30_000;

/** Options for constructing a {@link LinkMintClient}. Config is 12-factor: pass values from env. */
export interface LinkMintClientOptions {
  /** Base URL of the LinkMint API gateway, e.g. `https://api.linkmint.example`. */
  baseUrl: string;
  /** Auth passed through to the gateway (bearer JWT or partner API key). */
  auth?: AuthConfig;
  /** Custom fetch implementation. Defaults to the global `fetch` (Node 18+, browsers). */
  fetch?: FetchLike;
  /** Per-request timeout in milliseconds. Defaults to 30000. */
  timeoutMs?: number;
  /** Headers added to every request (overridden by per-call headers). */
  defaultHeaders?: Record<string, string>;
  /** Override the idempotency-key generator (defaults to a UUID v4). */
  generateIdempotencyKey?: () => string;
}

function normalizeBaseUrl(baseUrl: string): string {
  let parsed: URL;
  try {
    parsed = new URL(baseUrl);
  } catch {
    throw new LinkMintError(`invalid baseUrl: ${JSON.stringify(baseUrl)}`);
  }
  return parsed.toString().replace(/\/+$/, '');
}

function resolveConfig(options: LinkMintClientOptions): ResolvedConfig {
  if (!options.baseUrl) {
    throw new LinkMintError('baseUrl is required');
  }
  const provided = options.fetch ?? globalThis.fetch;
  if (typeof provided !== 'function') {
    throw new LinkMintError('no fetch implementation available; pass options.fetch');
  }
  // Bind the default global fetch to the global object: it is stored on `config` and later invoked
  // as `config.fetchImpl(...)`, which would otherwise re-bind `this` to `config` and make a browser's
  // native fetch throw "Illegal invocation". A caller-supplied fetch is used as-is.
  const fetchImpl = options.fetch ?? provided.bind(globalThis);
  return {
    baseUrl: normalizeBaseUrl(options.baseUrl),
    fetchImpl,
    auth: options.auth,
    timeoutMs: options.timeoutMs ?? DEFAULT_TIMEOUT_MS,
    defaultHeaders: options.defaultHeaders ?? {},
    generateIdempotencyKey: options.generateIdempotencyKey ?? defaultIdempotencyKey,
  };
}

/**
 * Typed client for the LinkMint `/v1` API.
 *
 * ```ts
 * const linkmint = new LinkMintClient({
 *   baseUrl: process.env.LINKMINT_API_URL!,
 *   auth: { type: 'bearer', token: process.env.LINKMINT_JWT! },
 * });
 * const link = await linkmint.paylinks.create({
 *   receiver: '0x1234...abcd',
 *   amount: 1000,
 *   expiry: new Date(Date.now() + 86_400_000),
 * });
 * const payment = await linkmint.payments.initiate({ paylink_id: link.pl_id, rail: 'mpesa' });
 * ```
 */
export class LinkMintClient {
  readonly paylinks: PayLinksResource;
  readonly payments: PaymentsResource;
  readonly notifications: NotificationsResource;

  constructor(options: LinkMintClientOptions) {
    const http = new HttpClient(resolveConfig(options));
    this.paylinks = new PayLinksResource(http);
    this.payments = new PaymentsResource(http);
    this.notifications = new NotificationsResource(http);
  }
}

/** Convenience factory equivalent to `new LinkMintClient(options)`. */
export function createClient(options: LinkMintClientOptions): LinkMintClient {
  return new LinkMintClient(options);
}
