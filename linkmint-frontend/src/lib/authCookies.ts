/**
 * Server-only helpers for the refresh-token cookie (`lm_rt`). The refresh token is httpOnly +
 * SameSite=Lax so browser JS can never read it (XSS-safe) and it rides same-origin requests to the
 * `/api/auth/*` handlers automatically. `Secure` is gated on production because browsers drop Secure
 * cookies over plain `http://localhost` in dev.
 *
 * Reads use the async `next/headers` cookies() store; writes are applied to the outgoing
 * `NextResponse` directly (unambiguous Set-Cookie attachment across Next versions).
 */

import 'server-only';

import { cookies } from 'next/headers';
import type { NextResponse } from 'next/server';

import { REFRESH_COOKIE } from './authSession';

const COOKIE_BASE = {
  httpOnly: true,
  secure: process.env.NODE_ENV === 'production',
  sameSite: 'lax' as const,
  path: '/',
};

/**
 * Cookie lifetime (seconds). identity-service enforces the refresh token's real TTL; this only
 * bounds how long the browser retains it. If the cookie outlives the server token, a refresh simply
 * 401s and forces re-login. Default 30 days; override via `LINKMINT_REFRESH_COOKIE_MAX_AGE_SECONDS`.
 */
function refreshMaxAge(): number {
  const raw = Number.parseInt(process.env.LINKMINT_REFRESH_COOKIE_MAX_AGE_SECONDS ?? '', 10);
  return Number.isFinite(raw) && raw > 0 ? raw : 60 * 60 * 24 * 30;
}

/** Set (or rotate) the refresh-token cookie on the response. */
export function setRefreshCookie(res: NextResponse, token: string): void {
  res.cookies.set(REFRESH_COOKIE, token, { ...COOKIE_BASE, maxAge: refreshMaxAge() });
}

/** Clear the refresh-token cookie on the response (logout, or a dead/expired refresh token). */
export function clearRefreshCookie(res: NextResponse): void {
  res.cookies.set(REFRESH_COOKIE, '', { ...COOKIE_BASE, maxAge: 0 });
}

/** Read the current refresh token from the request cookies, if any. */
export async function readRefreshCookie(): Promise<string | undefined> {
  const store = await cookies();
  return store.get(REFRESH_COOKIE)?.value;
}
