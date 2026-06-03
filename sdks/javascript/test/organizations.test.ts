import { describe, it, expect } from 'vitest';

import { LinkMintClient } from '../src/client';
import { ForbiddenError } from '../src/errors';
import { createMockFetch } from './helpers/mockFetch';

const BASE = 'https://api.linkmint.test';

describe('organizations.list', () => {
  it('GETs /v1/organizations and returns the caller orgs (no idempotency key, no creator addr)', async () => {
    const mock = createMockFetch({
      status: 200,
      body: {
        items: [
          {
            org_id: 'o-2',
            name: 'Beta',
            type: 'merchant',
            role: 'owner',
            created_at: '2026-02-01T00:00:00Z',
          },
          {
            org_id: 'o-1',
            name: 'Alpha',
            type: 'developer',
            role: 'viewer',
            created_at: '2026-01-01T00:00:00Z',
          },
        ],
      },
    });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    const result = await client.organizations.list();

    expect(result.items.map((o) => o.name)).toEqual(['Beta', 'Alpha']);
    const call = mock.lastCall();
    expect(call.method).toBe('GET');
    expect(call.url).toBe(`${BASE}/v1/organizations`);
    expect(call.headers['Idempotency-Key']).toBeUndefined();
    expect(call.headers['X-Creator-Addr']).toBeUndefined();
  });
});

describe('organizations.create', () => {
  it('POSTs /v1/organizations with the body and an idempotency key', async () => {
    const mock = createMockFetch({
      status: 201,
      body: {
        org_id: 'o-1',
        name: 'Acme',
        type: 'merchant',
        role: 'owner',
        created_at: '2026-01-01T00:00:00Z',
      },
    });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    const result = await client.organizations.create({ name: 'Acme', type: 'merchant' });

    expect(result.role).toBe('owner');
    const call = mock.lastCall();
    expect(call.method).toBe('POST');
    expect(call.url).toBe(`${BASE}/v1/organizations`);
    expect(call.body).toEqual({ name: 'Acme', type: 'merchant' });
    expect(call.headers['Idempotency-Key']).toBeTypeOf('string');
    expect(call.headers['X-Creator-Addr']).toBeUndefined();
  });
});

describe('organizations members', () => {
  it('POSTs a member by email and omits user_id', async () => {
    const mock = createMockFetch({
      status: 201,
      body: { org_id: 'o-1', user_id: 'u-2', role: 'developer' },
    });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    await client.organizations.addMember('o-1', { email: 'dev@b.com', role: 'developer' });
    const call = mock.lastCall();
    expect(call.url).toBe(`${BASE}/v1/organizations/o-1/members`);
    expect(call.body).toEqual({ role: 'developer', email: 'dev@b.com' });
  });

  it('maps a 403 FORBIDDEN add-member to ForbiddenError', async () => {
    const mock = createMockFetch({
      status: 403,
      body: { error: { code: 'FORBIDDEN', message: 'insufficient role' } },
    });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    await expect(
      client.organizations.addMember('o-1', { user_id: 'u-2', role: 'viewer' }),
    ).rejects.toBeInstanceOf(ForbiddenError);
  });

  it('GETs members (no idempotency key)', async () => {
    const mock = createMockFetch({ status: 200, body: { items: [] } });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    const result = await client.organizations.listMembers('o-1');
    expect(result.items).toEqual([]);
    const call = mock.lastCall();
    expect(call.method).toBe('GET');
    expect(call.url).toBe(`${BASE}/v1/organizations/o-1/members`);
    expect(call.headers['Idempotency-Key']).toBeUndefined();
  });

  it('DELETEs a member', async () => {
    const mock = createMockFetch({
      status: 200,
      body: { status: 'removed', org_id: 'o-1', user_id: 'u-2' },
    });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    const result = await client.organizations.removeMember('o-1', 'u-2');
    expect(result.status).toBe('removed');
    const call = mock.lastCall();
    expect(call.method).toBe('DELETE');
    expect(call.url).toBe(`${BASE}/v1/organizations/o-1/members/u-2`);
    expect(call.headers['Idempotency-Key']).toBeTypeOf('string');
  });
});
