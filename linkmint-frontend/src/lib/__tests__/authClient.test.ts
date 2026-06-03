/**
 * authClient.getAccessToken — the SDK token provider. Verifies: a fresh in-memory token is returned
 * without a network call; a missing/expired token triggers a SINGLE-FLIGHT refresh (concurrent callers
 * share one /api/auth/refresh); and a refresh 401 clears the session and rejects (→ reauth path).
 */

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';

import { confirmPasswordReset, getAccessToken, requestPasswordReset } from '@/lib/authClient';
import { useAuthStore } from '@/store/auth';

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'content-type': 'application/json' },
  });
}

beforeEach(() => {
  useAuthStore.setState({ status: 'unknown', user: null, accessToken: null, expiresAt: null });
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('authClient.getAccessToken', () => {
  it('returns the in-memory token when it is fresh (no refresh request)', async () => {
    const fetchSpy = vi.fn();
    vi.stubGlobal('fetch', fetchSpy);
    useAuthStore.setState({
      status: 'authed',
      accessToken: 'fresh-token',
      expiresAt: Date.now() + 120_000,
      user: null,
    });

    await expect(getAccessToken()).resolves.toBe('fresh-token');
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it('single-flights refresh: two concurrent calls trigger one /api/auth/refresh', async () => {
    const fetchSpy = vi
      .fn()
      .mockResolvedValue(
        jsonResponse({ accessToken: 'new-token', expiresIn: 900, expiresAt: Date.now() + 900_000 }),
      );
    vi.stubGlobal('fetch', fetchSpy);

    const [a, b] = await Promise.all([getAccessToken(), getAccessToken()]);

    expect(a).toBe('new-token');
    expect(b).toBe('new-token');
    expect(fetchSpy).toHaveBeenCalledTimes(1);
    expect(fetchSpy.mock.calls[0]?.[0]).toBe('/api/auth/refresh');
    expect(useAuthStore.getState().accessToken).toBe('new-token');
  });

  it('clears the session and rejects when refresh 401s (reuse/expiry)', async () => {
    const fetchSpy = vi
      .fn()
      .mockResolvedValue(jsonResponse({ error: { code: 'INVALID_TOKEN', message: 'reuse' } }, 401));
    vi.stubGlobal('fetch', fetchSpy);
    useAuthStore.setState({
      status: 'authed',
      accessToken: 'stale',
      expiresAt: Date.now() - 1,
      user: null,
    });

    await expect(getAccessToken()).rejects.toBeTruthy();
    expect(useAuthStore.getState().status).toBe('anon');
    expect(useAuthStore.getState().accessToken).toBeNull();
  });
});

describe('authClient password reset', () => {
  it('requestPasswordReset POSTs /api/auth/password/reset-request and returns the result', async () => {
    const fetchSpy = vi
      .fn()
      .mockResolvedValue(jsonResponse({ status: 'ok', reset_token: 'dev-token' }));
    vi.stubGlobal('fetch', fetchSpy);

    const result = await requestPasswordReset({ email: 'a@b.com' });

    expect(result).toEqual({ status: 'ok', reset_token: 'dev-token' });
    expect(fetchSpy.mock.calls[0]?.[0]).toBe('/api/auth/password/reset-request');
    const init = fetchSpy.mock.calls[0]?.[1] as RequestInit;
    expect(init.method).toBe('POST');
    expect(JSON.parse(init.body as string)).toEqual({ email: 'a@b.com' });
  });

  it('confirmPasswordReset POSTs /api/auth/password/reset-confirm', async () => {
    const fetchSpy = vi.fn().mockResolvedValue(jsonResponse({ status: 'reset' }));
    vi.stubGlobal('fetch', fetchSpy);

    await confirmPasswordReset({ token: 't-1', new_password: 'newpassw0rd' });

    expect(fetchSpy.mock.calls[0]?.[0]).toBe('/api/auth/password/reset-confirm');
    const init = fetchSpy.mock.calls[0]?.[1] as RequestInit;
    expect(JSON.parse(init.body as string)).toEqual({ token: 't-1', new_password: 'newpassw0rd' });
  });

  it('reconstructs the envelope into a real SDK error on a non-2xx (INVALID_TOKEN)', async () => {
    const fetchSpy = vi
      .fn()
      .mockResolvedValue(
        jsonResponse({ error: { code: 'INVALID_TOKEN', message: 'expired' } }, 401),
      );
    vi.stubGlobal('fetch', fetchSpy);

    await expect(
      confirmPasswordReset({ token: 'bad', new_password: 'newpassw0rd' }),
    ).rejects.toMatchObject({ code: 'INVALID_TOKEN', status: 401 });
  });
});
