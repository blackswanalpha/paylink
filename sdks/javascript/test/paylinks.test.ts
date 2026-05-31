import { describe, it, expect } from 'vitest';

import { LinkMintClient } from '../src/client';
import { BadRequestError, NotFoundError, UnauthorizedError } from '../src/errors';
import type { PayLink } from '../src/types';
import { createMockFetch } from './helpers/mockFetch';

const BASE = 'https://api.linkmint.test';

function samplePayLink(overrides: Partial<PayLink> = {}): PayLink {
  return {
    pl_id: '0x' + 'a'.repeat(64),
    creator: '0x' + '1'.repeat(40),
    receiver: '0x' + '2'.repeat(40),
    owner: '0x' + '1'.repeat(40),
    amount: 1000,
    currency: 'USD',
    status: 'CREATED',
    expiry: '2030-01-01T00:00:00Z',
    usage: 'single',
    vote_count: 0,
    chain_tx_hash: null,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    verified_at: null,
    ...overrides,
  };
}

describe('paylinks.create', () => {
  it('POSTs /v1/paylinks with the body, an auto idempotency key, and returns the typed result', async () => {
    const mock = createMockFetch({
      status: 201,
      body: {
        pl_id: '0x' + 'a'.repeat(64),
        status: 'CREATED',
        created_at: '2026-01-01T00:00:00Z',
        chain_tx_hash: null,
      },
    });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    const result = await client.paylinks.create({
      receiver: '0x' + '2'.repeat(40),
      amount: 1000,
      expiry: '2030-01-01T00:00:00Z',
    });

    expect(result.pl_id).toBe('0x' + 'a'.repeat(64));
    expect(result.status).toBe('CREATED');

    const call = mock.lastCall();
    expect(call.method).toBe('POST');
    expect(call.url).toBe(`${BASE}/v1/paylinks`);
    expect(call.headers['Content-Type']).toBe('application/json');
    expect(call.headers['Idempotency-Key']).toBeTypeOf('string');
    expect(call.headers['Idempotency-Key']?.length).toBeGreaterThan(0);
    expect(call.body).toEqual({
      receiver: '0x' + '2'.repeat(40),
      amount: 1000,
      expiry: '2030-01-01T00:00:00Z',
    });
  });

  it('serializes a Date expiry to ISO and includes optional fields when provided', async () => {
    const mock = createMockFetch({ status: 201, body: { pl_id: '0xabc', status: 'CREATED' } });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });
    const expiry = new Date('2030-06-01T12:00:00.000Z');

    await client.paylinks.create({
      receiver: '0x' + '2'.repeat(40),
      amount: 500,
      expiry,
      currency: 'PLN',
      usage: 'multi',
      metadata: { invoice: 'INV-1' },
      rules: { note: 'x' },
    });

    expect(mock.lastCall().body).toEqual({
      receiver: '0x' + '2'.repeat(40),
      amount: 500,
      expiry: '2030-06-01T12:00:00.000Z',
      currency: 'PLN',
      usage: 'multi',
      metadata: { invoice: 'INV-1' },
      rules: { note: 'x' },
    });
  });

  it('lets a caller supply their own Idempotency-Key', async () => {
    const mock = createMockFetch({ status: 201, body: { pl_id: '0xabc', status: 'CREATED' } });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    await client.paylinks.create(
      { receiver: '0x' + '2'.repeat(40), amount: 1, expiry: '2030-01-01T00:00:00Z' },
      { idempotencyKey: 'my-key-123' },
    );

    expect(mock.lastCall().headers['Idempotency-Key']).toBe('my-key-123');
  });

  it('never sends a client X-Creator-Addr header (gateway injects it — ADR-006)', async () => {
    const mock = createMockFetch({ status: 201, body: { pl_id: '0xabc', status: 'CREATED' } });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    await client.paylinks.create({
      receiver: '0x' + '2'.repeat(40),
      amount: 1,
      expiry: '2030-01-01T00:00:00Z',
    });

    expect(mock.lastCall().headers['X-Creator-Addr']).toBeUndefined();
  });

  it('maps a 400 INVALID_PAYLOAD envelope to BadRequestError', async () => {
    const mock = createMockFetch({
      status: 400,
      body: {
        error: {
          code: 'INVALID_PAYLOAD',
          message: 'request validation failed',
          details: { field: 'receiver' },
          trace_id: 'trace-1',
        },
      },
    });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    await expect(
      client.paylinks.create({ receiver: 'bad', amount: 1, expiry: '2030-01-01T00:00:00Z' }),
    ).rejects.toMatchObject({
      name: 'BadRequestError',
      status: 400,
      code: 'INVALID_PAYLOAD',
      details: { field: 'receiver' },
      traceId: 'trace-1',
    });
    await expect(
      client.paylinks.create({ receiver: 'bad', amount: 1, expiry: '2030-01-01T00:00:00Z' }),
    ).rejects.toBeInstanceOf(BadRequestError);
  });
});

describe('paylinks.get', () => {
  it('GETs /v1/paylinks/{pl_id} and returns the typed PayLink', async () => {
    const link = samplePayLink({ status: 'VERIFIED', vote_count: 3 });
    const mock = createMockFetch({ status: 200, body: link });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    const result = await client.paylinks.get(link.pl_id);

    expect(result).toEqual(link);
    const call = mock.lastCall();
    expect(call.method).toBe('GET');
    expect(call.url).toBe(`${BASE}/v1/paylinks/${link.pl_id}`);
    expect(call.headers['Idempotency-Key']).toBeUndefined();
  });

  it('maps a 404 PAYLINK_NOT_FOUND to NotFoundError', async () => {
    const mock = createMockFetch({
      status: 404,
      body: { error: { code: 'PAYLINK_NOT_FOUND', message: 'not found' } },
    });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    await expect(client.paylinks.get('0xmissing')).rejects.toBeInstanceOf(NotFoundError);
    await expect(client.paylinks.get('0xmissing')).rejects.toMatchObject({
      code: 'PAYLINK_NOT_FOUND',
      status: 404,
    });
  });
});

describe('paylinks.list', () => {
  it('builds the query string, omitting undefined filters, and returns items + next_cursor', async () => {
    const link = samplePayLink();
    const mock = createMockFetch({ status: 200, body: { items: [link], next_cursor: 'next-1' } });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    const result = await client.paylinks.list({
      creator: '0x' + '1'.repeat(40),
      status: 'CREATED',
      limit: 50,
    });

    expect(result.items).toHaveLength(1);
    expect(result.next_cursor).toBe('next-1');

    const url = new URL(mock.lastCall().url);
    expect(url.pathname).toBe('/v1/paylinks');
    expect(url.searchParams.get('creator')).toBe('0x' + '1'.repeat(40));
    expect(url.searchParams.get('status')).toBe('CREATED');
    expect(url.searchParams.get('limit')).toBe('50');
    expect(url.searchParams.has('receiver')).toBe(false);
    expect(url.searchParams.has('cursor')).toBe(false);
  });

  it('works with no params and a null next_cursor', async () => {
    const mock = createMockFetch({ status: 200, body: { items: [], next_cursor: null } });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    const result = await client.paylinks.list();
    expect(result.items).toEqual([]);
    expect(result.next_cursor).toBeNull();
    expect(new URL(mock.lastCall().url).search).toBe('');
  });

  it('maps a 400 INVALID_QUERY to BadRequestError', async () => {
    const mock = createMockFetch({
      status: 400,
      body: { error: { code: 'INVALID_QUERY', message: 'bad status filter' } },
    });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    await expect(client.paylinks.list({ status: 'CREATED' })).rejects.toMatchObject({
      name: 'BadRequestError',
      code: 'INVALID_QUERY',
    });
  });
});

describe('paylinks.cancel', () => {
  it('POSTs /v1/paylinks/{pl_id}/cancel with an idempotency key and returns CANCELLED', async () => {
    const plId = '0x' + 'a'.repeat(64);
    const mock = createMockFetch({ status: 200, body: { pl_id: plId, status: 'CANCELLED' } });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    const result = await client.paylinks.cancel(plId);

    expect(result).toEqual({ pl_id: plId, status: 'CANCELLED' });
    const call = mock.lastCall();
    expect(call.method).toBe('POST');
    expect(call.url).toBe(`${BASE}/v1/paylinks/${plId}/cancel`);
    expect(call.headers['Idempotency-Key']).toBeTypeOf('string');
    expect(call.body).toBeUndefined();
  });

  it('maps 401 UNAUTHORIZED to UnauthorizedError', async () => {
    const mock = createMockFetch({
      status: 401,
      body: { error: { code: 'UNAUTHORIZED', message: 'not owner' } },
    });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    await expect(client.paylinks.cancel('0xabc')).rejects.toBeInstanceOf(UnauthorizedError);
  });

  it('maps 409 PAYLINK_ALREADY_SETTLED to ConflictError', async () => {
    const mock = createMockFetch({
      status: 409,
      body: { error: { code: 'PAYLINK_ALREADY_SETTLED', message: 'already settled' } },
    });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    await expect(client.paylinks.cancel('0xabc')).rejects.toMatchObject({
      name: 'ConflictError',
      code: 'PAYLINK_ALREADY_SETTLED',
      status: 409,
    });
  });
});
