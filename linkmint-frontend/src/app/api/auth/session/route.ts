/**
 * GET /api/auth/session — cold-load session bootstrap.
 *
 * On a full page reload the in-memory access token is gone but the httpOnly refresh cookie persists.
 * This probe refreshes from that cookie and hydrates the profile, returning the full session; if
 * there's no cookie (or it's dead) it returns `{ authenticated: false }` (always HTTP 200 — absence
 * of a session is not an error). Rotates the cookie on success.
 */

import { NextResponse } from 'next/server';

import { clearRefreshCookie, readRefreshCookie, setRefreshCookie } from '@/lib/authCookies';
import { anonClient, bearerClient } from '@/lib/authServer';
import type { SessionProbe } from '@/lib/authSession';

export const dynamic = 'force-dynamic';

export async function GET(): Promise<NextResponse> {
  const refreshToken = await readRefreshCookie();
  if (!refreshToken) {
    return NextResponse.json({ authenticated: false } satisfies SessionProbe, { status: 200 });
  }

  try {
    const tokens = await anonClient().auth.refresh({ refresh_token: refreshToken });
    const user = await bearerClient(tokens.access_token).users.me();
    const probe: SessionProbe = {
      authenticated: true,
      accessToken: tokens.access_token,
      expiresIn: tokens.expires_in,
      expiresAt: Date.now() + tokens.expires_in * 1000,
      user,
    };
    const res = NextResponse.json(probe, { status: 200 });
    setRefreshCookie(res, tokens.refresh_token);
    return res;
  } catch {
    const res = NextResponse.json({ authenticated: false } satisfies SessionProbe, { status: 200 });
    clearRefreshCookie(res);
    return res;
  }
}
