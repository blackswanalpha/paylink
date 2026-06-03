import { describe, it, expect } from 'vitest';

import { LinkMintClient } from '../src/client';
import { ForbiddenError } from '../src/errors';
import { createMockFetch } from './helpers/mockFetch';

const BASE = 'https://api.linkmint.test';

const ENTRY = {
  entry_id: 7,
  occurred_at: '2026-01-01T00:00:00Z',
  actor: { id: 'u-1', kind: 'user' },
  action: 'merchant.onboard',
  resource: 'merchant:m-1',
  context: {},
  prev_hash: 'aa',
  entry_hash: 'bb',
};

describe('auditLog.list', () => {
  it('GETs /v1/audit-log with filters, omitting undefined params', async () => {
    const mock = createMockFetch({ status: 200, body: { items: [ENTRY], next_cursor: '6' } });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    const result = await client.auditLog.list({ resource: 'merchant:m-1', limit: 50 });

    expect(result.items[0]?.entry_id).toBe(7);
    expect(result.next_cursor).toBe('6');
    const url = new URL(mock.lastCall().url);
    expect(url.pathname).toBe('/v1/audit-log');
    expect(url.searchParams.get('resource')).toBe('merchant:m-1');
    expect(url.searchParams.get('limit')).toBe('50');
    expect(url.searchParams.has('actor')).toBe(false);
    expect(mock.lastCall().headers['Idempotency-Key']).toBeUndefined();
    expect(mock.lastCall().headers['X-Creator-Addr']).toBeUndefined();
  });

  it('maps a 403 FORBIDDEN (missing reader role) to ForbiddenError', async () => {
    const mock = createMockFetch({
      status: 403,
      body: { error: { code: 'FORBIDDEN', message: 'caller lacks an audit reader role' } },
    });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    await expect(client.auditLog.list()).rejects.toBeInstanceOf(ForbiddenError);
  });
});

describe('auditLog.get / verify', () => {
  it('GETs /v1/audit-log/{entry_id} (numeric id) and returns the entry + proof', async () => {
    const mock = createMockFetch({ status: 200, body: { entry: ENTRY, proof: { siblings: [] } } });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    const result = await client.auditLog.get(7);
    expect(result.entry.entry_id).toBe(7);
    expect(mock.lastCall().url).toBe(`${BASE}/v1/audit-log/7`);
  });

  it('GETs /v1/audit-log/verify with an optional range', async () => {
    const mock = createMockFetch({ status: 200, body: { ok: true } });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    const result = await client.auditLog.verify({ from: '2026-01-01T00:00:00Z' });
    expect(result.ok).toBe(true);
    const url = new URL(mock.lastCall().url);
    expect(url.pathname).toBe('/v1/audit-log/verify');
    expect(url.searchParams.get('from')).toBe('2026-01-01T00:00:00Z');
    expect(url.searchParams.has('to')).toBe(false);
  });

  it('surfaces broken_at when the chain is broken', async () => {
    const mock = createMockFetch({ status: 200, body: { ok: false, broken_at: 42 } });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    const result = await client.auditLog.verify();
    expect(result.ok).toBe(false);
    expect(result.broken_at).toBe(42);
  });
});
