/**
 * POST /api/auth/refresh — mint a fresh access token from the httpOnly refresh cookie.
 *
 * The hot path behind the SDK token provider: reads `lm_rt`, calls identity `auth.refresh` (which is
 * single-use with reuse-detection), ROTATES the cookie to the new refresh token, and returns just the
 * token half (no profile — the store already has the user). On failure (expired / reuse → 401) the
 * dead cookie is cleared so the client treats it as session-over and routes to `/login`.
 */

import { NextResponse } from 'next/server';

import { clearRefreshCookie, readRefreshCookie, setRefreshCookie } from '@/lib/authCookies';
import { anonClient } from '@/lib/authServer';
import { mapAuthError, type RefreshPayload } from '@/lib/authSession';

export const dynamic = 'force-dynamic';

export async function POST(): Promise<NextResponse> {
  const refreshToken = await readRefreshCookie();
  if (!refreshToken) {
    return NextResponse.json(
      { error: { code: 'INVALID_TOKEN', message: 'no active session', details: {} } },
      { status: 401 },
    );
  }

  try {
    const tokens = await anonClient().auth.refresh({ refresh_token: refreshToken });
    const payload: RefreshPayload = {
      accessToken: tokens.access_token,
      expiresIn: tokens.expires_in,
      expiresAt: Date.now() + tokens.expires_in * 1000,
    };
    const res = NextResponse.json(payload, { status: 200 });
    setRefreshCookie(res, tokens.refresh_token);
    return res;
  } catch (err) {
    const { status, body: envelope } = mapAuthError(err);
    const res = NextResponse.json(envelope, { status });
    clearRefreshCookie(res);
    return res;
  }
}
