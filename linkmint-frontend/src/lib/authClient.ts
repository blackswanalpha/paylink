'use client';

/**
 * Browser-side auth orchestration for work09/10. Talks to the `/api/auth/*` route handlers (which own
 * the httpOnly refresh cookie) and keeps `useAuthStore` in sync. Exposes `getAccessToken` — the SDK
 * bearer-token provider that transparently refreshes a near-expiry token, single-flighted so a burst
 * of concurrent SDK calls triggers exactly one refresh.
 *
 * Non-2xx responses are reconstructed into real `LinkMintApiError`s so `reportError`/`classifyError`
 * treat them exactly like a direct SDK call (the login form branches on `MFA_REQUIRED`).
 */

import {
  createApiError,
  isErrorEnvelope,
  type PasswordResetRequestResult,
  type RegisterResult,
} from '@linkmint/sdk';

import type {
  LoginRequestBody,
  PasswordResetConfirmBody,
  PasswordResetRequestBody,
  RefreshPayload,
  RegisterRequestBody,
  SessionPayload,
  SessionProbe,
} from '@/lib/authSession';
import { useAuthStore } from '@/store/auth';

/** Refresh a token this many ms before it actually expires (clock-skew / in-flight cushion). */
const TOKEN_SKEW_MS = 30_000;

/** Fetch an `/api/auth/*` endpoint; on a non-2xx envelope, throw a reconstructed SDK error. */
async function authFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...(init?.headers ?? {}) },
  });
  const data: unknown = await res.json().catch(() => null);
  if (!res.ok) {
    if (isErrorEnvelope(data)) {
      throw createApiError({
        status: res.status,
        code: data.error.code,
        message: data.error.message,
        details: data.error.details ?? {},
        traceId: data.error.trace_id,
        requestId: res.headers.get('x-request-id') ?? undefined,
      });
    }
    throw createApiError({
      status: res.status,
      code: 'INTERNAL_ERROR',
      message: 'the request failed',
      details: {},
      traceId: undefined,
      requestId: undefined,
    });
  }
  return data as T;
}

/** Log in (optionally with an MFA code). Throws an `MFA_REQUIRED`/`MFA_INVALID`/… SDK error on failure. */
export async function login(body: LoginRequestBody): Promise<void> {
  const payload = await authFetch<SessionPayload>('/api/auth/login', {
    method: 'POST',
    body: JSON.stringify(body),
  });
  useAuthStore.getState().setSession({
    accessToken: payload.accessToken,
    expiresAt: payload.expiresAt,
    user: payload.user,
  });
}

/** Register a new user. Does not log in — the caller redirects to `/login` on success. */
export async function register(body: RegisterRequestBody): Promise<RegisterResult> {
  return authFetch<RegisterResult>('/api/auth/register', {
    method: 'POST',
    body: JSON.stringify(body),
  });
}

/**
 * Request a password reset. Anti-enumeration: resolves the same way whether or not the account
 * exists; the caller shows an identical confirmation. In dev the result may carry `reset_token`.
 */
export async function requestPasswordReset(
  body: PasswordResetRequestBody,
): Promise<PasswordResetRequestResult> {
  return authFetch<PasswordResetRequestResult>('/api/auth/password/reset-request', {
    method: 'POST',
    body: JSON.stringify(body),
  });
}

/** Complete a password reset with a reset token. Throws an `INVALID_TOKEN`/… SDK error on failure. */
export async function confirmPasswordReset(body: PasswordResetConfirmBody): Promise<void> {
  await authFetch('/api/auth/password/reset-confirm', {
    method: 'POST',
    body: JSON.stringify(body),
  });
}

/** Log out: revoke server-side (best-effort) then clear the local session regardless. */
export async function logout(): Promise<void> {
  const { accessToken } = useAuthStore.getState();
  try {
    await authFetch('/api/auth/logout', {
      method: 'POST',
      headers: accessToken ? { Authorization: `Bearer ${accessToken}` } : {},
    });
  } catch {
    // Best-effort — the local session is cleared below no matter what.
  } finally {
    useAuthStore.getState().clearSession();
  }
}

let bootstrapPromise: Promise<void> | null = null;

/** One-shot cold-load rehydrate from the refresh cookie. Idempotent while in flight. */
export function bootstrapSession(): Promise<void> {
  if (bootstrapPromise) {
    return bootstrapPromise;
  }
  const run = (async () => {
    try {
      const probe = await authFetch<SessionProbe>('/api/auth/session', { method: 'GET' });
      if (probe.authenticated) {
        useAuthStore.getState().setSession({
          accessToken: probe.accessToken,
          expiresAt: probe.expiresAt,
          user: probe.user,
        });
      } else {
        useAuthStore.getState().clearSession();
      }
    } catch {
      useAuthStore.getState().clearSession();
    } finally {
      bootstrapPromise = null; // allow re-bootstrap after this settles (e.g. logout → login)
    }
  })();
  bootstrapPromise = run;
  return run;
}

let refreshPromise: Promise<string> | null = null;

/** Single-flight refresh: one `/api/auth/refresh` regardless of how many callers race here. */
function refreshAccessToken(): Promise<string> {
  if (refreshPromise) {
    return refreshPromise;
  }
  const run = (async () => {
    try {
      const payload = await authFetch<RefreshPayload>('/api/auth/refresh', { method: 'POST' });
      useAuthStore.getState().setToken({
        accessToken: payload.accessToken,
        expiresAt: payload.expiresAt,
      });
      return payload.accessToken;
    } catch (err) {
      // Refresh failed (expired / reuse-detected) → the session is over.
      useAuthStore.getState().clearSession();
      throw err;
    } finally {
      refreshPromise = null; // reset the single-flight latch once settled
    }
  })();
  refreshPromise = run;
  return run;
}

/**
 * The SDK bearer-token provider. Returns the in-memory access token, or transparently refreshes it
 * when it's missing / within the skew window of expiry. A thrown error here propagates as the SDK
 * call's failure (a dead session surfaces as a 401 → the reauth overlay).
 */
export async function getAccessToken(): Promise<string> {
  const { accessToken, expiresAt } = useAuthStore.getState();
  if (accessToken && expiresAt && expiresAt - Date.now() > TOKEN_SKEW_MS) {
    return accessToken;
  }
  return refreshAccessToken();
}
