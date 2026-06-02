import { describe, it, expect } from 'vitest';

import { LinkMintClient } from '../src/client';
import { NotFoundError } from '../src/errors';
import type { Notification } from '../src/types';
import { createMockFetch } from './helpers/mockFetch';

const BASE = 'https://api.linkmint.test';

function sample(overrides: Partial<Notification> = {}): Notification {
  return {
    id: '11111111-1111-1111-1111-111111111111',
    kind: 'success',
    title: 'PayLink settled',
    body: 'PayLink 0xpl was verified on-chain.',
    href: '/dashboard/paylinks',
    read: false,
    created_at: '2026-06-02T00:00:00Z',
    ...overrides,
  };
}

describe('notifications.list', () => {
  it('GETs /v1/notifications with limit + cursor and returns items + next_cursor', async () => {
    const mock = createMockFetch({
      status: 200,
      body: { items: [sample()], next_cursor: 'next-1' },
    });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    const result = await client.notifications.list({ limit: 20, cursor: 'c0' });

    expect(result.items).toHaveLength(1);
    expect(result.items.map((n) => n.kind)).toEqual(['success']);
    expect(result.next_cursor).toBe('next-1');

    const call = mock.lastCall();
    const url = new URL(call.url);
    expect(call.method).toBe('GET');
    expect(url.pathname).toBe('/v1/notifications');
    expect(url.searchParams.get('limit')).toBe('20');
    expect(url.searchParams.get('cursor')).toBe('c0');
  });

  it('omits absent query params', async () => {
    const mock = createMockFetch({ status: 200, body: { items: [], next_cursor: null } });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    await client.notifications.list();

    const url = new URL(mock.lastCall().url);
    expect(url.searchParams.has('limit')).toBe(false);
    expect(url.searchParams.has('cursor')).toBe(false);
  });
});

describe('notifications.markRead', () => {
  it('POSTs /v1/notifications/{id}/read with an auto idempotency key', async () => {
    const id = '11111111-1111-1111-1111-111111111111';
    const mock = createMockFetch({ status: 200, body: sample({ read: true }) });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    const result = await client.notifications.markRead(id);

    expect(result.read).toBe(true);
    const call = mock.lastCall();
    expect(call.method).toBe('POST');
    expect(new URL(call.url).pathname).toBe(`/v1/notifications/${id}/read`);
    expect(call.headers['Idempotency-Key']).toBeTypeOf('string');
  });

  it('maps 404 NOTIFICATION_NOT_FOUND to NotFoundError', async () => {
    const mock = createMockFetch({
      status: 404,
      body: { error: { code: 'NOTIFICATION_NOT_FOUND', message: 'not found' } },
    });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    await expect(client.notifications.markRead('missing')).rejects.toBeInstanceOf(NotFoundError);
  });
});

describe('notifications.markAllRead', () => {
  it('POSTs /v1/notifications/read-all and returns the count', async () => {
    const mock = createMockFetch({ status: 200, body: { count: 3 } });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    const result = await client.notifications.markAllRead();

    expect(result.count).toBe(3);
    const call = mock.lastCall();
    expect(call.method).toBe('POST');
    expect(new URL(call.url).pathname).toBe('/v1/notifications/read-all');
    expect(call.headers['Idempotency-Key']).toBeTypeOf('string');
  });
});
