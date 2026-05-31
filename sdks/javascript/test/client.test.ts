import { describe, it, expect, vi, afterEach } from 'vitest';

import { createClient, LinkMintClient } from '../src/client';
import { LinkMintError } from '../src/errors';
import type { FetchLike } from '../src/http';
import { createMockFetch } from './helpers/mockFetch';

const BASE = 'https://api.linkmint.test';

describe('client construction', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('createClient returns a LinkMintClient with resource namespaces', () => {
    const client = createClient({ baseUrl: BASE, fetch: createMockFetch({}).fetch });
    expect(client).toBeInstanceOf(LinkMintClient);
    expect(client.paylinks).toBeDefined();
    expect(client.payments).toBeDefined();
  });

  it('throws when baseUrl is missing', () => {
    expect(() => new LinkMintClient({ baseUrl: '' })).toThrow(LinkMintError);
    expect(() => new LinkMintClient({ baseUrl: '' })).toThrow('baseUrl is required');
  });

  it('throws when baseUrl is not a valid URL', () => {
    expect(
      () => new LinkMintClient({ baseUrl: 'not a url', fetch: createMockFetch({}).fetch }),
    ).toThrow(/invalid baseUrl/);
  });

  it('throws when no fetch implementation is available', () => {
    vi.stubGlobal('fetch', undefined);
    expect(() => new LinkMintClient({ baseUrl: BASE })).toThrow(/no fetch implementation/);
  });

  it('uses the global fetch when none is supplied', async () => {
    const mock = createMockFetch({ status: 200, body: { items: [], next_cursor: null } });
    vi.stubGlobal('fetch', mock.fetch);
    const client = new LinkMintClient({ baseUrl: BASE });
    await client.paylinks.list();
    expect(mock.calls).toHaveLength(1);
  });

  it('invokes the default global fetch with a valid `this` (regression: native fetch Illegal invocation)', async () => {
    // A browser's native fetch throws "Illegal invocation" when called with a `this` that is not the
    // global object. Use a non-arrow stub so it observes `this`: before the fix the SDK stored
    // `globalThis.fetch` unbound and called it as `config.fetchImpl(...)`, so `this` was the config
    // object and every request failed in the browser (but not under a `this`-agnostic mock).
    const mock = createMockFetch({ status: 200, body: { items: [], next_cursor: null } });
    const nativeLike = function (
      this: unknown,
      ...args: Parameters<FetchLike>
    ): ReturnType<FetchLike> {
      if (this !== undefined && this !== globalThis) {
        throw new TypeError("Failed to execute 'fetch' on 'Window': Illegal invocation");
      }
      return mock.fetch(...args);
    };
    vi.stubGlobal('fetch', nativeLike);
    const client = new LinkMintClient({ baseUrl: BASE });
    await expect(client.paylinks.list()).resolves.toBeDefined();
    expect(mock.calls).toHaveLength(1);
  });

  it('normalizes a trailing slash on the base URL (no double slash)', async () => {
    const mock = createMockFetch({ status: 200, body: { items: [], next_cursor: null } });
    const client = new LinkMintClient({ baseUrl: `${BASE}/`, fetch: mock.fetch });
    await client.paylinks.list();
    expect(mock.lastCall().url).toBe(`${BASE}/v1/paylinks`);
  });

  it('applies default headers to requests', async () => {
    const mock = createMockFetch({ status: 200, body: { items: [], next_cursor: null } });
    const client = new LinkMintClient({
      baseUrl: BASE,
      fetch: mock.fetch,
      defaultHeaders: { 'X-Client': 'web-app' },
    });
    await client.paylinks.list();
    expect(mock.lastCall().headers['X-Client']).toBe('web-app');
  });
});

describe('end-to-end create -> initiate -> settle', () => {
  it('drives a PayLink from creation to a settled payment, with bearer auth on every call', async () => {
    const plId = '0x' + 'a'.repeat(64);
    const mock = createMockFetch((req) => {
      if (req.method === 'POST' && req.url.endsWith('/v1/paylinks')) {
        return {
          status: 201,
          body: { pl_id: plId, status: 'CREATED', created_at: 't0', chain_tx_hash: null },
        };
      }
      if (req.method === 'POST' && req.url.endsWith('/v1/payments')) {
        return {
          status: 201,
          body: {
            id: 'pay-1',
            paylink_id: plId,
            rail: 'mpesa',
            status: 'AWAITING_PAYMENT',
            created_at: 't1',
            updated_at: 't1',
          },
        };
      }
      if (req.method === 'GET' && req.url.endsWith('/v1/payments/pay-1')) {
        return {
          status: 200,
          body: {
            id: 'pay-1',
            paylink_id: plId,
            rail: 'mpesa',
            status: 'SETTLED',
            created_at: 't1',
            updated_at: 't2',
          },
        };
      }
      return { status: 404, body: { error: { code: 'NOT_FOUND', message: 'no route' } } };
    });

    const client = createClient({
      baseUrl: BASE,
      auth: { type: 'bearer', token: 'jwt-xyz' },
      fetch: mock.fetch,
    });

    const link = await client.paylinks.create({
      receiver: '0x' + '2'.repeat(40),
      amount: 1000,
      expiry: '2030-01-01T00:00:00Z',
    });
    expect(link.status).toBe('CREATED');

    const payment = await client.payments.initiate({ paylink_id: link.pl_id, rail: 'mpesa' });
    expect(payment.status).toBe('AWAITING_PAYMENT');

    const settled = await client.payments.get(payment.id);
    expect(settled.status).toBe('SETTLED');

    expect(mock.calls).toHaveLength(3);
    for (const call of mock.calls) {
      expect(call.headers['Authorization']).toBe('Bearer jwt-xyz');
    }
  });
});
