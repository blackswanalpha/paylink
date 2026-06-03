import { describe, it, expect } from 'vitest';

import { LinkMintClient } from '../src/client';
import { ConflictError, UnauthorizedError } from '../src/errors';
import { createMockFetch } from './helpers/mockFetch';

const BASE = 'https://api.linkmint.test';

const TOKENS = {
  access_token: 'access-1',
  refresh_token: 'refresh-1',
  token_type: 'Bearer',
  expires_in: 900,
};

describe('auth.register', () => {
  it('POSTs /v1/auth/register with the body, an auto idempotency key, and no X-Creator-Addr', async () => {
    const mock = createMockFetch({ status: 201, body: { user_id: 'u-1', status: 'ACTIVE' } });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    const result = await client.auth.register({ email: 'a@b.com', password: 'hunter2hunter2' });

    expect(result).toEqual({ user_id: 'u-1', status: 'ACTIVE' });
    const call = mock.lastCall();
    expect(call.method).toBe('POST');
    expect(call.url).toBe(`${BASE}/v1/auth/register`);
    expect(call.body).toEqual({ email: 'a@b.com', password: 'hunter2hunter2' });
    expect(call.headers['Idempotency-Key']).toBeTypeOf('string');
    expect(call.headers['X-Creator-Addr']).toBeUndefined();
  });

  it('includes phone when provided', async () => {
    const mock = createMockFetch({ status: 201, body: { user_id: 'u-2', status: 'ACTIVE' } });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    await client.auth.register({ phone: '+254700000000', password: 'hunter2hunter2' });
    expect(mock.lastCall().body).toEqual({ phone: '+254700000000', password: 'hunter2hunter2' });
  });

  it('maps a 409 IDEMPOTENT_CONFLICT to ConflictError', async () => {
    const mock = createMockFetch({
      status: 409,
      body: { error: { code: 'IDEMPOTENT_CONFLICT', message: 'in progress' } },
    });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    await expect(
      client.auth.register({ email: 'a@b.com', password: 'x'.repeat(8) }),
    ).rejects.toBeInstanceOf(ConflictError);
  });
});

describe('auth.login / refresh', () => {
  it('POSTs /v1/auth/login and returns the token pair', async () => {
    const mock = createMockFetch({ status: 200, body: TOKENS });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    const result = await client.auth.login({
      email: 'a@b.com',
      password: 'pw',
      mfa_code: '123456',
    });

    expect(result).toEqual(TOKENS);
    const call = mock.lastCall();
    expect(call.url).toBe(`${BASE}/v1/auth/login`);
    expect(call.body).toEqual({ email: 'a@b.com', password: 'pw', mfa_code: '123456' });
  });

  it('maps a 401 UNAUTHORIZED login to UnauthorizedError', async () => {
    const mock = createMockFetch({
      status: 401,
      body: { error: { code: 'UNAUTHORIZED', message: 'bad credentials' } },
    });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    await expect(client.auth.login({ email: 'a@b.com', password: 'nope' })).rejects.toBeInstanceOf(
      UnauthorizedError,
    );
  });

  it('POSTs /v1/auth/refresh with the refresh token', async () => {
    const mock = createMockFetch({ status: 200, body: TOKENS });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    await client.auth.refresh({ refresh_token: 'refresh-1' });
    const call = mock.lastCall();
    expect(call.url).toBe(`${BASE}/v1/auth/refresh`);
    expect(call.body).toEqual({ refresh_token: 'refresh-1' });
    expect(call.headers['Idempotency-Key']).toBeTypeOf('string');
  });
});

describe('auth.logout / oauth / mfa', () => {
  it('POSTs /v1/auth/logout', async () => {
    const mock = createMockFetch({ status: 200, body: { status: 'logged_out' } });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    const result = await client.auth.logout({ refresh_token: 'refresh-1' });
    expect(result).toEqual({ status: 'logged_out' });
    expect(mock.lastCall().url).toBe(`${BASE}/v1/auth/logout`);
  });

  it('POSTs /v1/auth/oauth/{provider}/start with optional fields', async () => {
    const mock = createMockFetch({
      status: 200,
      body: { authorize_url: 'https://idp/authorize', state: 's-1' },
    });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    const result = await client.auth.oauthStart('google', { redirect_uri: 'https://app/cb' });
    expect(result.authorize_url).toBe('https://idp/authorize');
    const call = mock.lastCall();
    expect(call.url).toBe(`${BASE}/v1/auth/oauth/google/start`);
    expect(call.body).toEqual({ redirect_uri: 'https://app/cb' });
  });

  it('POSTs /v1/auth/oauth/{provider}/callback and returns tokens', async () => {
    const mock = createMockFetch({ status: 200, body: TOKENS });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    await client.auth.oauthCallback('google', { code: 'abc', state: 's-1' });
    const call = mock.lastCall();
    expect(call.url).toBe(`${BASE}/v1/auth/oauth/google/callback`);
    expect(call.body).toEqual({ code: 'abc', state: 's-1' });
  });

  it('POSTs /v1/auth/password/reset-request with the identifier (no X-Creator-Addr)', async () => {
    const mock = createMockFetch({ status: 200, body: { status: 'ok', reset_token: null } });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    const result = await client.auth.requestPasswordReset({ email: 'a@b.com' });
    expect(result).toEqual({ status: 'ok', reset_token: null });
    const call = mock.lastCall();
    expect(call.method).toBe('POST');
    expect(call.url).toBe(`${BASE}/v1/auth/password/reset-request`);
    expect(call.body).toEqual({ email: 'a@b.com' });
    expect(call.headers['Idempotency-Key']).toBeTypeOf('string');
    expect(call.headers['X-Creator-Addr']).toBeUndefined();
  });

  it('POSTs /v1/auth/password/reset-confirm with the token + new password', async () => {
    const mock = createMockFetch({ status: 200, body: { status: 'reset' } });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    const result = await client.auth.confirmPasswordReset({
      token: 't-1',
      new_password: 'newpassw0rd',
    });
    expect(result).toEqual({ status: 'reset' });
    const call = mock.lastCall();
    expect(call.url).toBe(`${BASE}/v1/auth/password/reset-confirm`);
    expect(call.body).toEqual({ token: 't-1', new_password: 'newpassw0rd' });
  });

  it('maps a 401 INVALID_TOKEN reset-confirm to UnauthorizedError', async () => {
    const mock = createMockFetch({
      status: 401,
      body: { error: { code: 'INVALID_TOKEN', message: 'invalid or expired reset token' } },
    });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    await expect(
      client.auth.confirmPasswordReset({ token: 'bad', new_password: 'newpassw0rd' }),
    ).rejects.toBeInstanceOf(UnauthorizedError);
  });

  it('POSTs /v1/auth/mfa/enroll (no body) and verify/disable (code body)', async () => {
    const enroll = createMockFetch({
      status: 200,
      body: { secret: 'S', otpauth_uri: 'otpauth://x' },
    });
    const c1 = new LinkMintClient({ baseUrl: BASE, fetch: enroll.fetch });
    const enrolled = await c1.auth.mfaEnroll();
    expect(enrolled.secret).toBe('S');
    expect(enroll.lastCall().url).toBe(`${BASE}/v1/auth/mfa/enroll`);
    expect(enroll.lastCall().body).toBeUndefined();
    expect(enroll.lastCall().headers['Idempotency-Key']).toBeTypeOf('string');

    const verify = createMockFetch({ status: 200, body: { enabled: true } });
    const c2 = new LinkMintClient({ baseUrl: BASE, fetch: verify.fetch });
    const v = await c2.auth.mfaVerify({ code: '123456' });
    expect(v.enabled).toBe(true);
    expect(verify.lastCall().body).toEqual({ code: '123456' });

    const disable = createMockFetch({ status: 200, body: { status: 'disabled' } });
    const c3 = new LinkMintClient({ baseUrl: BASE, fetch: disable.fetch });
    const d = await c3.auth.mfaDisable({ code: '654321' });
    expect(d.status).toBe('disabled');
    expect(disable.lastCall().url).toBe(`${BASE}/v1/auth/mfa/disable`);
  });
});
