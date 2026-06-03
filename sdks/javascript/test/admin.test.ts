import { describe, it, expect } from 'vitest';

import { LinkMintClient } from '../src/client';
import { ForbiddenError } from '../src/errors';
import { createMockFetch } from './helpers/mockFetch';

const BASE = 'https://api.linkmint.test';

describe('admin.search', () => {
  it('GETs /v1/admin/search with the q query and returns grouped hits', async () => {
    const mock = createMockFetch({
      status: 200,
      body: {
        query: 'acme',
        groups: {
          merchant: [
            {
              type: 'merchant',
              id: 'm-1',
              label: 'Acme',
              status: 'ACTIVE',
              secondary: { country: 'KE' },
            },
          ],
        },
        degraded: [],
      },
    });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    const result = await client.admin.search('acme');

    expect(result.groups.merchant?.[0]?.id).toBe('m-1');
    const call = mock.lastCall();
    expect(call.method).toBe('GET');
    const url = new URL(call.url);
    expect(url.pathname).toBe('/v1/admin/search');
    expect(url.searchParams.get('q')).toBe('acme');
    expect(call.headers['Idempotency-Key']).toBeUndefined();
    expect(call.headers['X-Creator-Addr']).toBeUndefined();
  });

  it('maps a 403 MFA_REQUIRED to ForbiddenError (status-mapped)', async () => {
    const mock = createMockFetch({
      status: 403,
      body: { error: { code: 'MFA_REQUIRED', message: 'MFA required for admin access' } },
    });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    await expect(client.admin.search('acme')).rejects.toMatchObject({ code: 'MFA_REQUIRED' });
    await expect(client.admin.search('acme')).rejects.toBeInstanceOf(ForbiddenError);
  });
});

describe('admin entity drill-down', () => {
  it('GETs each entity type at /v1/admin/{type}/{id}', async () => {
    for (const [method, type] of [
      ['getUser', 'users'],
      ['getMerchant', 'merchants'],
      ['getPaylink', 'paylinks'],
      ['getPayment', 'payments'],
    ] as const) {
      const mock = createMockFetch({ status: 200, body: { type, id: 'x-1', data: {} } });
      const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

      const result = await client.admin[method]('x-1');
      expect(result.type).toBe(type);
      const call = mock.lastCall();
      expect(call.method).toBe('GET');
      expect(call.url).toBe(`${BASE}/v1/admin/${type}/x-1`);
      expect(call.headers['Idempotency-Key']).toBeUndefined();
    }
  });

  it('getEntity builds the path from the entity type', async () => {
    const mock = createMockFetch({
      status: 200,
      body: { type: 'users', id: 'u-1', data: { email: 'a@b.com' } },
    });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    const result = await client.admin.getEntity('users', 'u-1');
    expect(result.data.email).toBe('a@b.com');
    expect(mock.lastCall().url).toBe(`${BASE}/v1/admin/users/u-1`);
  });
});
