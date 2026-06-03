import { describe, it, expect } from 'vitest';

import { LinkMintClient } from '../src/client';
import { UnauthorizedError } from '../src/errors';
import type { UserProfile } from '../src/types';
import { createMockFetch } from './helpers/mockFetch';

const BASE = 'https://api.linkmint.test';

function sampleProfile(overrides: Partial<UserProfile> = {}): UserProfile {
  return {
    user_id: 'u-1',
    email: 'a@b.com',
    phone: null,
    kyc_tier: 1,
    status: 'ACTIVE',
    mfa_enabled: false,
    roles: [{ org_id: 'o-1', role: 'owner' }],
    user_roles: [],
    created_at: '2026-01-01T00:00:00Z',
    last_login_at: null,
    ...overrides,
  };
}

describe('users.me', () => {
  it('GETs /v1/users/me with no idempotency key and returns the typed profile', async () => {
    const profile = sampleProfile();
    const mock = createMockFetch({ status: 200, body: profile });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    const result = await client.users.me();

    expect(result).toEqual(profile);
    const call = mock.lastCall();
    expect(call.method).toBe('GET');
    expect(call.url).toBe(`${BASE}/v1/users/me`);
    expect(call.headers['Idempotency-Key']).toBeUndefined();
    expect(call.headers['X-Creator-Addr']).toBeUndefined();
  });

  it('maps a 401 UNAUTHORIZED to UnauthorizedError', async () => {
    const mock = createMockFetch({
      status: 401,
      body: { error: { code: 'UNAUTHORIZED', message: 'missing token' } },
    });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    await expect(client.users.me()).rejects.toBeInstanceOf(UnauthorizedError);
  });
});

describe('users.updateMe', () => {
  it('PATCHes /v1/users/me with the provided fields and an idempotency key', async () => {
    const mock = createMockFetch({ status: 200, body: sampleProfile({ phone: '+254700000000' }) });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    await client.users.updateMe({ phone: '+254700000000' });

    const call = mock.lastCall();
    expect(call.method).toBe('PATCH');
    expect(call.url).toBe(`${BASE}/v1/users/me`);
    expect(call.body).toEqual({ phone: '+254700000000' });
    expect(call.headers['Idempotency-Key']).toBeTypeOf('string');
  });
});

describe('users API keys', () => {
  it('POSTs /v1/users/me/api-keys and returns the one-time full_key', async () => {
    const mock = createMockFetch({
      status: 201,
      body: {
        api_key_id: 'k-1',
        org_id: 'o-1',
        name: 'ci',
        prefix: 'lm_live_abcd',
        full_key: 'lm_live_abcd.secret',
        scopes: ['paylinks:read'],
        status: 'ACTIVE',
        created_at: '2026-01-01T00:00:00Z',
      },
    });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    const result = await client.users.createApiKey({
      org_id: 'o-1',
      name: 'ci',
      scopes: ['paylinks:read'],
    });

    expect(result.full_key).toBe('lm_live_abcd.secret');
    const call = mock.lastCall();
    expect(call.url).toBe(`${BASE}/v1/users/me/api-keys`);
    expect(call.body).toEqual({ org_id: 'o-1', name: 'ci', scopes: ['paylinks:read'] });
    expect(call.headers['Idempotency-Key']).toBeTypeOf('string');
  });

  it('omits scopes from the body when not provided', async () => {
    const mock = createMockFetch({ status: 201, body: { api_key_id: 'k-2' } });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    await client.users.createApiKey({ org_id: 'o-1', name: 'ci' });
    expect(mock.lastCall().body).toEqual({ org_id: 'o-1', name: 'ci' });
  });

  it('GETs /v1/users/me/api-keys', async () => {
    const mock = createMockFetch({ status: 200, body: { items: [] } });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    const result = await client.users.listApiKeys();
    expect(result.items).toEqual([]);
    expect(mock.lastCall().method).toBe('GET');
    expect(mock.lastCall().headers['Idempotency-Key']).toBeUndefined();
  });

  it('DELETEs /v1/users/me/api-keys/{id} with an idempotency key', async () => {
    const mock = createMockFetch({ status: 200, body: { api_key_id: 'k-1', status: 'REVOKED' } });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    const result = await client.users.revokeApiKey('k-1');
    expect(result).toEqual({ api_key_id: 'k-1', status: 'REVOKED' });
    const call = mock.lastCall();
    expect(call.method).toBe('DELETE');
    expect(call.url).toBe(`${BASE}/v1/users/me/api-keys/k-1`);
    expect(call.headers['Idempotency-Key']).toBeTypeOf('string');
  });
});
