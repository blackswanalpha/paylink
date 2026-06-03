import { describe, it, expect } from 'vitest';

import { LinkMintClient } from '../src/client';
import { ForbiddenError, NotFoundError } from '../src/errors';
import { createMockFetch } from './helpers/mockFetch';

const BASE = 'https://api.linkmint.test';

describe('compliance.status', () => {
  it('GETs /v1/compliance/status with the user_id query and no idempotency key', async () => {
    const mock = createMockFetch({
      status: 200,
      body: {
        user_id: 'u-1',
        kyc_tier: 1,
        risk_score: 12.5,
        flags: [{ kind: 'velocity', severity: 'low', raised_at: '2026-01-01T00:00:00Z' }],
      },
    });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    const result = await client.compliance.status('u-1');

    expect(result.kyc_tier).toBe(1);
    expect(result.flags[0]?.kind).toBe('velocity');
    const call = mock.lastCall();
    expect(call.method).toBe('GET');
    const url = new URL(call.url);
    expect(url.pathname).toBe('/v1/compliance/status');
    expect(url.searchParams.get('user_id')).toBe('u-1');
    expect(call.headers['Idempotency-Key']).toBeUndefined();
    expect(call.headers['X-Creator-Addr']).toBeUndefined();
  });

  it('maps a 403 FORBIDDEN (reading another user) to ForbiddenError', async () => {
    const mock = createMockFetch({
      status: 403,
      body: { error: { code: 'FORBIDDEN', message: 'self or admin only' } },
    });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    await expect(client.compliance.status('someone-else')).rejects.toBeInstanceOf(ForbiddenError);
  });

  it('maps a 404 COMPLIANCE_NOT_FOUND to NotFoundError', async () => {
    const mock = createMockFetch({
      status: 404,
      body: { error: { code: 'COMPLIANCE_NOT_FOUND', message: 'unknown user' } },
    });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    await expect(client.compliance.status('u-1')).rejects.toMatchObject({
      code: 'COMPLIANCE_NOT_FOUND',
    });
    await expect(client.compliance.status('u-1')).rejects.toBeInstanceOf(NotFoundError);
  });
});

describe('compliance.createKycSession', () => {
  it('POSTs /v1/kyc/sessions with the body and an idempotency key', async () => {
    const mock = createMockFetch({
      status: 201,
      body: { session_id: 'k-1', provider_url: 'https://veriff/session/k-1' },
    });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    const result = await client.compliance.createKycSession({ user_id: 'u-1', tier_requested: 2 });

    expect(result.provider_url).toContain('veriff');
    const call = mock.lastCall();
    expect(call.method).toBe('POST');
    expect(call.url).toBe(`${BASE}/v1/kyc/sessions`);
    expect(call.body).toEqual({ user_id: 'u-1', tier_requested: 2 });
    expect(call.headers['Idempotency-Key']).toBeTypeOf('string');
  });
});
