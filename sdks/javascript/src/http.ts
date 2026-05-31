/**
 * Low-level HTTP transport for the LinkMint SDK: URL/query building, auth header injection,
 * idempotency-key handling, request timeout, and mapping the standard error envelope to typed
 * errors. Resources ({@link ./resources}) call this; application code does not use it directly.
 */

import type { LinkMintApiError } from './errors';
import {
  createApiError,
  isErrorEnvelope,
  LinkMintConnectionError,
  LinkMintError,
  LinkMintTimeoutError,
  type ErrorCode,
} from './errors';

/** The subset of the Fetch API the SDK depends on. Matches the global `fetch` signature. */
export type FetchLike = typeof fetch;

/** Authentication passed through to the API gateway. Exactly one mechanism. */
export type AuthConfig =
  | {
      type: 'bearer';
      /** A JWT string, or a (possibly async) provider invoked per request for token refresh. */
      token: string | (() => string | Promise<string>);
    }
  | {
      type: 'apiKey';
      /** Partner API key, sent as the `X-API-Key` header. */
      key: string;
    };

/** Per-request options accepted by every resource method. */
export interface RequestOptions {
  /** Override the auto-generated `Idempotency-Key` (mutating calls only). */
  idempotencyKey?: string;
  /** Abort signal to cancel the request. */
  signal?: AbortSignal;
  /** Correlation id sent as `X-Request-Id`; echoed back by the gateway and used in error traces. */
  requestId?: string;
  /** Extra headers merged onto this request (lowest precedence is SDK defaults). */
  headers?: Record<string, string>;
}

/** Fully-resolved transport configuration (produced by the client from its options). */
export interface ResolvedConfig {
  baseUrl: string;
  fetchImpl: FetchLike;
  auth: AuthConfig | undefined;
  timeoutMs: number;
  defaultHeaders: Record<string, string>;
  generateIdempotencyKey: () => string;
}

/** An internal, fully-specified HTTP request built by a resource method. */
export interface HttpRequest {
  method: 'GET' | 'POST';
  path: string;
  query?: Record<string, string | number | undefined>;
  body?: unknown;
  /** When set, sent as the `Idempotency-Key` header. */
  idempotencyKey?: string;
}

const REQUEST_ID_HEADER = 'X-Request-Id';

export class HttpClient {
  constructor(private readonly config: ResolvedConfig) {}

  /** Generate a fresh idempotency key using the configured generator. */
  newIdempotencyKey(): string {
    return this.config.generateIdempotencyKey();
  }

  /** Execute a request and decode the JSON response as `T`, or throw a typed error. */
  async request<T>(req: HttpRequest, options: RequestOptions = {}): Promise<T> {
    const url = this.buildUrl(req.path, req.query);
    const headers = await this.buildHeaders(req, options);

    const init: RequestInit = {
      method: req.method,
      headers,
    };
    const body = serializeBody(req.body);
    if (body !== undefined) {
      init.body = body;
    }

    const controller = new AbortController();
    let timedOut = false;
    const timer = setTimeout(() => {
      timedOut = true;
      controller.abort();
    }, this.config.timeoutMs);
    const detach = linkSignal(options.signal, controller);
    init.signal = controller.signal;

    let response: Response;
    try {
      // Invoke through a local binding, never as `this.config.fetchImpl(...)`: a method call would
      // set `this` to the config object and make a native (unbound) fetch throw "Illegal invocation".
      const fetchImpl = this.config.fetchImpl;
      response = await fetchImpl(url, init);
    } catch (err) {
      if (timedOut) {
        throw new LinkMintTimeoutError(this.config.timeoutMs);
      }
      if (options.signal?.aborted) {
        throw new LinkMintConnectionError('request aborted by caller', {
          cause: options.signal.reason,
        });
      }
      throw new LinkMintConnectionError('network request failed', { cause: err });
    } finally {
      clearTimeout(timer);
      detach();
    }

    if (!response.ok) {
      throw await errorFromResponse(response);
    }
    return decodeJson<T>(response);
  }

  private buildUrl(path: string, query?: Record<string, string | number | undefined>): string {
    const url = new URL(this.config.baseUrl + path);
    if (query) {
      for (const [key, value] of Object.entries(query)) {
        if (value !== undefined && value !== '') {
          url.searchParams.set(key, String(value));
        }
      }
    }
    return url.toString();
  }

  private async buildHeaders(
    req: HttpRequest,
    options: RequestOptions,
  ): Promise<Record<string, string>> {
    const headers: Record<string, string> = {
      Accept: 'application/json',
      ...this.config.defaultHeaders,
      ...options.headers,
    };
    if (req.body !== undefined) {
      headers['Content-Type'] = 'application/json';
    }
    await applyAuth(headers, this.config.auth);

    const idempotencyKey = options.idempotencyKey ?? req.idempotencyKey;
    if (idempotencyKey !== undefined) {
      headers['Idempotency-Key'] = idempotencyKey;
    }
    if (options.requestId !== undefined) {
      headers[REQUEST_ID_HEADER] = options.requestId;
    }
    return headers;
  }
}

/** Apply the configured auth mechanism to the outgoing headers. */
async function applyAuth(
  headers: Record<string, string>,
  auth: AuthConfig | undefined,
): Promise<void> {
  if (!auth) {
    return;
  }
  if (auth.type === 'bearer') {
    // A throwing/rejecting token provider is the caller's own error; let it propagate
    // unwrapped so its original cause and type are preserved.
    const token = typeof auth.token === 'function' ? await auth.token() : auth.token;
    headers['Authorization'] = `Bearer ${token}`;
  } else {
    headers['X-API-Key'] = auth.key;
  }
}

/** Bridge a caller-supplied AbortSignal to the internal controller; returns a detach function. */
function linkSignal(signal: AbortSignal | undefined, controller: AbortController): () => void {
  if (!signal) {
    return () => {};
  }
  if (signal.aborted) {
    controller.abort();
    return () => {};
  }
  const onAbort = (): void => controller.abort();
  signal.addEventListener('abort', onAbort, { once: true });
  return () => signal.removeEventListener('abort', onAbort);
}

/** Serialize a request body to JSON, surfacing a non-serializable body as a typed SDK error. */
function serializeBody(body: unknown): string | undefined {
  if (body === undefined) {
    return undefined;
  }
  try {
    return JSON.stringify(body);
  } catch (err) {
    throw new LinkMintError('failed to serialize request body to JSON', { cause: err });
  }
}

/** Decode a successful response body as `T`; an empty body resolves to `undefined`. */
async function decodeJson<T>(response: Response): Promise<T> {
  if (response.status === 204) {
    return undefined as T;
  }
  const text = await response.text();
  if (text.length === 0) {
    return undefined as T;
  }
  return JSON.parse(text) as T;
}

/** Parse an error response into the most specific typed error available. */
export async function errorFromResponse(response: Response): Promise<LinkMintApiError> {
  const requestId = response.headers.get('x-request-id') ?? undefined;
  const retryAfter = parseRetryAfter(response.headers.get('retry-after'));

  let code: ErrorCode = `HTTP_${response.status}`;
  let message = response.statusText || 'request failed';
  let details: Record<string, unknown> = {};
  let traceId: string | undefined = requestId;

  const bodyText = await safeText(response);
  if (bodyText.length > 0) {
    const parsed = tryParseJson(bodyText);
    if (isErrorEnvelope(parsed)) {
      code = parsed.error.code || code;
      message = parsed.error.message || message;
      details = parsed.error.details ?? {};
      traceId = parsed.error.trace_id ?? traceId;
    } else {
      details = { body: bodyText };
    }
  }

  return createApiError({
    status: response.status,
    code,
    message,
    details,
    traceId,
    requestId,
    retryAfter,
  });
}

async function safeText(response: Response): Promise<string> {
  try {
    return await response.text();
  } catch {
    return '';
  }
}

function tryParseJson(text: string): unknown {
  try {
    return JSON.parse(text) as unknown;
  } catch {
    return undefined;
  }
}

function parseRetryAfter(raw: string | null): number | undefined {
  if (raw === null) {
    return undefined;
  }
  const seconds = Number.parseInt(raw, 10);
  return Number.isNaN(seconds) ? undefined : seconds;
}
