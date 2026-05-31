/**
 * Typed errors for the LinkMint SDK.
 *
 * Every `>= 400` API response uses the standard LinkMint envelope:
 *
 *     { "error": { "code": "...", "message": "...", "details": {}, "trace_id": "..." } }
 *
 * The SDK parses that envelope into a {@link LinkMintApiError} (with a typed {@link ErrorCode})
 * and a status-mapped subclass so callers can branch with `instanceof` or on `.code`. Transport
 * failures (no HTTP response) surface as {@link LinkMintConnectionError} / {@link LinkMintTimeoutError}.
 */

/** Machine-readable error codes known to the LinkMint `/v1` API + gateway. */
export type KnownErrorCode =
  // paylink-service (app/errors.py) + shared
  | 'PAYLINK_NOT_FOUND'
  | 'INVALID_PAYLOAD'
  | 'INVALID_QUERY'
  | 'IDEMPOTENT_CONFLICT'
  | 'PAYLINK_ALREADY_SETTLED'
  | 'PAYLINK_EXPIRED'
  | 'KYC_REQUIRED'
  | 'CHAIN_UNAVAILABLE'
  | 'UNAUTHORIZED'
  | 'INTERNAL_ERROR'
  // payment-orchestrator (internal/httpx/envelope.go)
  | 'PAYMENT_NOT_FOUND'
  | 'PAYMENT_EXISTS'
  | 'PAYLINK_NOT_PAYABLE'
  | 'PAYLINK_SERVICE_UNAVAILABLE'
  | 'SERVICE_NOT_READY'
  // api-gateway (Kong) normalized codes
  | 'NOT_FOUND'
  | 'RATE_LIMITED'
  | 'BAD_GATEWAY'
  | 'SERVICE_UNAVAILABLE';

/**
 * Error code as carried in the envelope. Known codes get autocompletion; unknown future codes
 * are still accepted (the `& {}` keeps the literal union open without widening to bare `string`).
 */
export type ErrorCode = KnownErrorCode | (string & {});

/** The exact shape of the LinkMint error envelope. */
export interface ErrorEnvelope {
  error: {
    code: string;
    message: string;
    details?: Record<string, unknown>;
    trace_id?: string;
  };
}

/** Base class for every error thrown by the SDK. */
export class LinkMintError extends Error {
  constructor(message: string, options?: ErrorOptions) {
    super(message, options);
    this.name = 'LinkMintError';
    // Keep `instanceof` working when targeting older runtimes / down-leveled output.
    Object.setPrototypeOf(this, new.target.prototype);
  }
}

/** A transport-level failure: the request never produced an HTTP response. */
export class LinkMintConnectionError extends LinkMintError {
  constructor(message: string, options?: ErrorOptions) {
    super(message, options);
    this.name = 'LinkMintConnectionError';
  }
}

/** The request exceeded the configured timeout and was aborted. */
export class LinkMintTimeoutError extends LinkMintConnectionError {
  readonly timeoutMs: number;
  constructor(timeoutMs: number) {
    super(`request timed out after ${timeoutMs}ms`);
    this.name = 'LinkMintTimeoutError';
    this.timeoutMs = timeoutMs;
  }
}

/** Fields used to construct a {@link LinkMintApiError}. */
export interface ApiErrorInit {
  status: number;
  code: ErrorCode;
  message: string;
  details: Record<string, unknown>;
  traceId: string | undefined;
  requestId: string | undefined;
  retryAfter?: number;
}

/** An error returned by the API as a structured envelope (HTTP status >= 400). */
export class LinkMintApiError extends LinkMintError {
  /** HTTP status code of the response. */
  readonly status: number;
  /** Machine-readable error code from the envelope. */
  readonly code: ErrorCode;
  /** Structured, code-specific detail object (may be empty). */
  readonly details: Record<string, unknown>;
  /** Correlation id from the envelope `trace_id`, when present. */
  readonly traceId: string | undefined;
  /** Correlation id from the response `X-Request-Id` header, when present. */
  readonly requestId: string | undefined;

  constructor(init: ApiErrorInit) {
    super(init.message);
    this.name = 'LinkMintApiError';
    this.status = init.status;
    this.code = init.code;
    this.details = init.details;
    this.traceId = init.traceId;
    this.requestId = init.requestId;
  }
}

/** HTTP 400 — `INVALID_PAYLOAD` / `INVALID_QUERY`. */
export class BadRequestError extends LinkMintApiError {
  constructor(init: ApiErrorInit) {
    super(init);
    this.name = 'BadRequestError';
  }
}

/** HTTP 401 — `UNAUTHORIZED`. */
export class UnauthorizedError extends LinkMintApiError {
  constructor(init: ApiErrorInit) {
    super(init);
    this.name = 'UnauthorizedError';
  }
}

/** HTTP 402 — `KYC_REQUIRED`. */
export class PaymentRequiredError extends LinkMintApiError {
  constructor(init: ApiErrorInit) {
    super(init);
    this.name = 'PaymentRequiredError';
  }
}

/** HTTP 403. */
export class ForbiddenError extends LinkMintApiError {
  constructor(init: ApiErrorInit) {
    super(init);
    this.name = 'ForbiddenError';
  }
}

/** HTTP 404 — `PAYLINK_NOT_FOUND` / `PAYMENT_NOT_FOUND` / `NOT_FOUND`. */
export class NotFoundError extends LinkMintApiError {
  constructor(init: ApiErrorInit) {
    super(init);
    this.name = 'NotFoundError';
  }
}

/**
 * HTTP 409 — `IDEMPOTENT_CONFLICT`, `PAYLINK_ALREADY_SETTLED`, `PAYLINK_EXPIRED`,
 * `PAYMENT_EXISTS`, `PAYLINK_NOT_PAYABLE`.
 */
export class ConflictError extends LinkMintApiError {
  constructor(init: ApiErrorInit) {
    super(init);
    this.name = 'ConflictError';
  }
}

/** HTTP 429 — `RATE_LIMITED`. Exposes the `Retry-After` value (seconds) when present. */
export class RateLimitError extends LinkMintApiError {
  readonly retryAfter: number | undefined;
  constructor(init: ApiErrorInit) {
    super(init);
    this.name = 'RateLimitError';
    this.retryAfter = init.retryAfter;
  }
}

/** HTTP 5xx — `INTERNAL_ERROR`, `CHAIN_UNAVAILABLE`, `*_UNAVAILABLE`, `SERVICE_NOT_READY`, `BAD_GATEWAY`. */
export class ServerError extends LinkMintApiError {
  constructor(init: ApiErrorInit) {
    super(init);
    this.name = 'ServerError';
  }
}

/** Build the most specific {@link LinkMintApiError} subclass for the response status. */
export function createApiError(init: ApiErrorInit): LinkMintApiError {
  switch (init.status) {
    case 400:
      return new BadRequestError(init);
    case 401:
      return new UnauthorizedError(init);
    case 402:
      return new PaymentRequiredError(init);
    case 403:
      return new ForbiddenError(init);
    case 404:
      return new NotFoundError(init);
    case 409:
      return new ConflictError(init);
    case 429:
      return new RateLimitError(init);
    default:
      if (init.status >= 500) {
        return new ServerError(init);
      }
      return new LinkMintApiError(init);
  }
}

/** Type guard: `true` when `err` is any {@link LinkMintApiError}. */
export function isLinkMintApiError(err: unknown): err is LinkMintApiError {
  return err instanceof LinkMintApiError;
}

/** Type guard: structural check that `value` is a LinkMint error envelope. */
export function isErrorEnvelope(value: unknown): value is ErrorEnvelope {
  if (typeof value !== 'object' || value === null) {
    return false;
  }
  const errorObj = (value as { error?: unknown }).error;
  if (typeof errorObj !== 'object' || errorObj === null) {
    return false;
  }
  const { code, message } = errorObj as { code?: unknown; message?: unknown };
  return typeof code === 'string' && typeof message === 'string';
}
