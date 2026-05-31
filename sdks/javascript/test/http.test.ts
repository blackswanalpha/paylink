import { describe, it, expect } from 'vitest';

import type { LinkMintApiError } from '../src/errors';
import {
  LinkMintConnectionError,
  LinkMintError,
  LinkMintTimeoutError,
  RateLimitError,
  ServerError,
} from '../src/errors';
import { HttpClient, type FetchLike, type ResolvedConfig } from '../src/http';
import { createMockFetch } from './helpers/mockFetch';

const BASE = 'https://api.linkmint.test';

function makeClient(fetchImpl: FetchLike, overrides: Partial<ResolvedConfig> = {}): HttpClient {
  return new HttpClient({
    baseUrl: BASE,
    fetchImpl,
    auth: undefined,
    timeoutMs: 30_000,
    defaultHeaders: {},
    generateIdempotencyKey: () => 'fixed-key',
    ...overrides,
  });
}

/** A fetch that hangs until aborted, then rejects like the real Fetch API. */
const abortableHang: FetchLike = (_input, init) =>
  new Promise((_resolve, reject) => {
    const signal = init?.signal ?? undefined;
    const onAbort = (): void => reject(new DOMException('aborted', 'AbortError'));
    if (signal?.aborted) {
      onAbort();
      return;
    }
    signal?.addEventListener('abort', onAbort, { once: true });
  });

describe('auth pass-through', () => {
  it('sends a bearer token from a string', async () => {
    const mock = createMockFetch({ status: 200, body: {} });
    const client = makeClient(mock.fetch, { auth: { type: 'bearer', token: 'jwt-abc' } });
    await client.request({ method: 'GET', path: '/v1/x' });
    expect(mock.lastCall().headers['Authorization']).toBe('Bearer jwt-abc');
    expect(mock.lastCall().headers['X-API-Key']).toBeUndefined();
  });

  it('resolves a bearer token from an async provider', async () => {
    const mock = createMockFetch({ status: 200, body: {} });
    const client = makeClient(mock.fetch, {
      auth: { type: 'bearer', token: () => Promise.resolve('fresh-token') },
    });
    await client.request({ method: 'GET', path: '/v1/x' });
    expect(mock.lastCall().headers['Authorization']).toBe('Bearer fresh-token');
  });

  it('sends an API key as X-API-Key', async () => {
    const mock = createMockFetch({ status: 200, body: {} });
    const client = makeClient(mock.fetch, { auth: { type: 'apiKey', key: 'partner-key' } });
    await client.request({ method: 'GET', path: '/v1/x' });
    expect(mock.lastCall().headers['X-API-Key']).toBe('partner-key');
    expect(mock.lastCall().headers['Authorization']).toBeUndefined();
  });

  it('sends no auth header when unconfigured', async () => {
    const mock = createMockFetch({ status: 200, body: {} });
    const client = makeClient(mock.fetch);
    await client.request({ method: 'GET', path: '/v1/x' });
    expect(mock.lastCall().headers['Authorization']).toBeUndefined();
    expect(mock.lastCall().headers['X-API-Key']).toBeUndefined();
  });
});

describe('headers', () => {
  it('merges default headers and per-call headers, with per-call taking precedence', async () => {
    const mock = createMockFetch({ status: 200, body: {} });
    const client = makeClient(mock.fetch, { defaultHeaders: { 'X-App': 'web', 'X-Keep': 'yes' } });
    await client.request({ method: 'GET', path: '/v1/x' }, { headers: { 'X-App': 'override' } });
    const { headers } = mock.lastCall();
    expect(headers['Accept']).toBe('application/json');
    expect(headers['X-App']).toBe('override');
    expect(headers['X-Keep']).toBe('yes');
  });

  it('sets X-Request-Id from the requestId option', async () => {
    const mock = createMockFetch({ status: 200, body: {} });
    const client = makeClient(mock.fetch);
    await client.request({ method: 'GET', path: '/v1/x' }, { requestId: 'corr-1' });
    expect(mock.lastCall().headers['X-Request-Id']).toBe('corr-1');
  });

  it('uses the request idempotency key, and lets the option override it', async () => {
    const mock = createMockFetch([
      { status: 200, body: {} },
      { status: 200, body: {} },
    ]);
    const client = makeClient(mock.fetch);
    await client.request({ method: 'POST', path: '/v1/x', body: {}, idempotencyKey: 'req-key' });
    expect(mock.calls[0]?.headers['Idempotency-Key']).toBe('req-key');
    await client.request(
      { method: 'POST', path: '/v1/x', body: {}, idempotencyKey: 'req-key' },
      { idempotencyKey: 'opt-key' },
    );
    expect(mock.calls[1]?.headers['Idempotency-Key']).toBe('opt-key');
  });
});

describe('url + query building', () => {
  it('omits undefined and empty query values and stringifies numbers', async () => {
    const mock = createMockFetch({ status: 200, body: {} });
    const client = makeClient(mock.fetch);
    await client.request({
      method: 'GET',
      path: '/v1/x',
      query: { a: 'v', b: undefined, c: '', n: 7 },
    });
    const url = new URL(mock.lastCall().url);
    expect(url.searchParams.get('a')).toBe('v');
    expect(url.searchParams.get('n')).toBe('7');
    expect(url.searchParams.has('b')).toBe(false);
    expect(url.searchParams.has('c')).toBe(false);
  });
});

describe('response decoding', () => {
  it('returns undefined for an empty 200 body', async () => {
    const mock = createMockFetch({ status: 200 });
    const client = makeClient(mock.fetch);
    await expect(client.request({ method: 'GET', path: '/v1/x' })).resolves.toBeUndefined();
  });

  it('returns undefined for a 204', async () => {
    const mock = createMockFetch({ status: 204 });
    const client = makeClient(mock.fetch);
    await expect(client.request({ method: 'GET', path: '/v1/x' })).resolves.toBeUndefined();
  });

  it('parses a JSON body', async () => {
    const mock = createMockFetch({ status: 200, body: { hello: 'world' } });
    const client = makeClient(mock.fetch);
    await expect(client.request({ method: 'GET', path: '/v1/x' })).resolves.toEqual({
      hello: 'world',
    });
  });
});

describe('transport failures', () => {
  it('throws LinkMintTimeoutError when the request exceeds the timeout', async () => {
    const client = makeClient(abortableHang, { timeoutMs: 10 });
    await expect(client.request({ method: 'GET', path: '/v1/x' })).rejects.toBeInstanceOf(
      LinkMintTimeoutError,
    );
  });

  it('throws LinkMintConnectionError (not timeout) when the caller aborts', async () => {
    const controller = new AbortController();
    const client = makeClient(abortableHang, { timeoutMs: 1000 });
    const promise = client.request({ method: 'GET', path: '/v1/x' }, { signal: controller.signal });
    controller.abort();
    await expect(promise).rejects.toBeInstanceOf(LinkMintConnectionError);
    await expect(promise).rejects.not.toBeInstanceOf(LinkMintTimeoutError);
  });

  it('aborts immediately when the supplied signal is already aborted', async () => {
    const client = makeClient(abortableHang, { timeoutMs: 1000 });
    const signal = AbortSignal.abort();
    await expect(
      client.request({ method: 'GET', path: '/v1/x' }, { signal }),
    ).rejects.toBeInstanceOf(LinkMintConnectionError);
  });

  it('throws a typed LinkMintError when the request body is not serializable, without calling fetch', async () => {
    const mock = createMockFetch({ status: 200, body: {} });
    const client = makeClient(mock.fetch);
    await expect(
      client.request({ method: 'POST', path: '/v1/x', body: { bad: 10n } }),
    ).rejects.toBeInstanceOf(LinkMintError);
    expect(mock.calls).toHaveLength(0);
  });

  it('wraps a network error as LinkMintConnectionError with the cause attached', async () => {
    const boom: FetchLike = () => Promise.reject(new TypeError('boom'));
    const client = makeClient(boom);
    await expect(client.request({ method: 'GET', path: '/v1/x' })).rejects.toMatchObject({
      name: 'LinkMintConnectionError',
    });
    const err = await client.request({ method: 'GET', path: '/v1/x' }).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(LinkMintConnectionError);
    expect((err as LinkMintConnectionError).cause).toBeInstanceOf(TypeError);
  });
});

describe('error-envelope mapping', () => {
  it('falls back to the response body in details when the body is not an envelope', async () => {
    const mock = createMockFetch({ status: 500, rawBody: 'upstream is on fire' });
    const client = makeClient(mock.fetch);
    const err = await client.request({ method: 'GET', path: '/v1/x' }).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ServerError);
    const apiErr = err as ServerError;
    expect(apiErr.code).toBe('HTTP_500');
    expect(apiErr.details).toEqual({ body: 'upstream is on fire' });
  });

  it('uses the X-Request-Id header as the trace id when the envelope omits trace_id', async () => {
    const mock = createMockFetch({
      status: 404,
      headers: { 'X-Request-Id': 'rid-77' },
      body: { error: { code: 'PAYLINK_NOT_FOUND', message: 'gone' } },
    });
    const client = makeClient(mock.fetch);
    const err = (await client
      .request({ method: 'GET', path: '/v1/x' })
      .catch((e: unknown) => e)) as LinkMintApiError;
    expect(err.requestId).toBe('rid-77');
    expect(err.traceId).toBe('rid-77');
  });

  it('parses Retry-After into RateLimitError.retryAfter', async () => {
    const mock = createMockFetch({
      status: 429,
      headers: { 'Retry-After': '45' },
      body: { error: { code: 'RATE_LIMITED', message: 'slow down' } },
    });
    const client = makeClient(mock.fetch);
    const err = (await client
      .request({ method: 'GET', path: '/v1/x' })
      .catch((e: unknown) => e)) as RateLimitError;
    expect(err).toBeInstanceOf(RateLimitError);
    expect(err.retryAfter).toBe(45);
  });

  it('leaves retryAfter undefined for a non-numeric Retry-After', async () => {
    const mock = createMockFetch({
      status: 429,
      headers: { 'Retry-After': 'soon' },
      body: { error: { code: 'RATE_LIMITED', message: 'slow down' } },
    });
    const client = makeClient(mock.fetch);
    const err = (await client
      .request({ method: 'GET', path: '/v1/x' })
      .catch((e: unknown) => e)) as RateLimitError;
    expect(err.retryAfter).toBeUndefined();
  });

  it('handles an empty error body using the status text', async () => {
    const mock = createMockFetch({ status: 502 });
    const client = makeClient(mock.fetch);
    const err = (await client
      .request({ method: 'GET', path: '/v1/x' })
      .catch((e: unknown) => e)) as ServerError;
    expect(err).toBeInstanceOf(ServerError);
    expect(err.code).toBe('HTTP_502');
    expect(err.details).toEqual({});
  });
});
