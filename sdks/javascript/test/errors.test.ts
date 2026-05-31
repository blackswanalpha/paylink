import { describe, it, expect } from 'vitest';

import {
  BadRequestError,
  ConflictError,
  createApiError,
  ForbiddenError,
  isErrorEnvelope,
  isLinkMintApiError,
  LinkMintApiError,
  LinkMintConnectionError,
  LinkMintError,
  LinkMintTimeoutError,
  NotFoundError,
  PaymentRequiredError,
  RateLimitError,
  ServerError,
  UnauthorizedError,
  type ApiErrorInit,
} from '../src/errors';

function init(status: number, extra: Partial<ApiErrorInit> = {}): ApiErrorInit {
  return {
    status,
    code: 'INTERNAL_ERROR',
    message: 'msg',
    details: {},
    traceId: undefined,
    requestId: undefined,
    ...extra,
  };
}

describe('createApiError', () => {
  it.each([
    [400, BadRequestError, 'BadRequestError'],
    [401, UnauthorizedError, 'UnauthorizedError'],
    [402, PaymentRequiredError, 'PaymentRequiredError'],
    [403, ForbiddenError, 'ForbiddenError'],
    [404, NotFoundError, 'NotFoundError'],
    [409, ConflictError, 'ConflictError'],
    [429, RateLimitError, 'RateLimitError'],
    [500, ServerError, 'ServerError'],
    [502, ServerError, 'ServerError'],
    [503, ServerError, 'ServerError'],
  ] as const)('maps status %i to %s', (status, ctor, name) => {
    const err = createApiError(init(status));
    expect(err).toBeInstanceOf(ctor);
    expect(err).toBeInstanceOf(LinkMintApiError);
    expect(err).toBeInstanceOf(LinkMintError);
    expect(err).toBeInstanceOf(Error);
    expect(err.name).toBe(name);
    expect(err.status).toBe(status);
  });

  it('falls back to the base LinkMintApiError for an unmapped 4xx status', () => {
    const err = createApiError(init(418));
    expect(err.constructor).toBe(LinkMintApiError);
    expect(err.name).toBe('LinkMintApiError');
  });

  it('exposes retryAfter on RateLimitError', () => {
    const err = createApiError(init(429, { code: 'RATE_LIMITED', retryAfter: 30 }));
    expect(err).toBeInstanceOf(RateLimitError);
    expect((err as RateLimitError).retryAfter).toBe(30);
  });

  it('carries code, details, traceId, and requestId through', () => {
    const err = createApiError(
      init(404, {
        code: 'PAYLINK_NOT_FOUND',
        message: 'gone',
        details: { pl_id: '0xabc' },
        traceId: 'trace-9',
        requestId: 'req-9',
      }),
    );
    expect(err.code).toBe('PAYLINK_NOT_FOUND');
    expect(err.message).toBe('gone');
    expect(err.details).toEqual({ pl_id: '0xabc' });
    expect(err.traceId).toBe('trace-9');
    expect(err.requestId).toBe('req-9');
  });
});

describe('isLinkMintApiError', () => {
  it('is true for API errors and false otherwise', () => {
    expect(isLinkMintApiError(createApiError(init(400)))).toBe(true);
    expect(isLinkMintApiError(new LinkMintConnectionError('x'))).toBe(false);
    expect(isLinkMintApiError(new Error('x'))).toBe(false);
    expect(isLinkMintApiError(null)).toBe(false);
  });
});

describe('isErrorEnvelope', () => {
  it('accepts a well-formed envelope', () => {
    expect(isErrorEnvelope({ error: { code: 'X', message: 'm' } })).toBe(true);
  });

  it('rejects malformed values', () => {
    expect(isErrorEnvelope(null)).toBe(false);
    expect(isErrorEnvelope('nope')).toBe(false);
    expect(isErrorEnvelope({})).toBe(false);
    expect(isErrorEnvelope({ error: null })).toBe(false);
    expect(isErrorEnvelope({ error: { code: 1, message: 'm' } })).toBe(false);
    expect(isErrorEnvelope({ error: { code: 'X' } })).toBe(false);
  });
});

describe('connection errors', () => {
  it('LinkMintTimeoutError carries the timeout and is a connection error', () => {
    const err = new LinkMintTimeoutError(5000);
    expect(err.timeoutMs).toBe(5000);
    expect(err).toBeInstanceOf(LinkMintConnectionError);
    expect(err).toBeInstanceOf(LinkMintError);
    expect(err.name).toBe('LinkMintTimeoutError');
  });

  it('LinkMintError preserves a cause', () => {
    const cause = new Error('root');
    const err = new LinkMintError('wrapped', { cause });
    expect(err.cause).toBe(cause);
  });
});
