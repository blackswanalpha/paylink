/**
 * POST /api/auth/logout — end the session.
 *
 * Best-effort: if both the refresh cookie and the caller's access token (sent as `Authorization`)
 * are present, revoke the refresh token on identity-service; then ALWAYS clear the cookie and return
 * 200. Idempotent — succeeds even if the token is already dead.
 */

import { NextResponse } from 'next/server';

import { clearRefreshCookie, readRefreshCookie } from '@/lib/authCookies';
import { bearerClient } from '@/lib/authServer';

export const dynamic = 'force-dynamic';

export async function POST(req: Request): Promise<NextResponse> {
  const refreshToken = await readRefreshCookie();
  const authz = req.headers.get('authorization') ?? '';
  const accessToken = authz.toLowerCase().startsWith('bearer ') ? authz.slice(7).trim() : '';

  if (refreshToken && accessToken) {
    try {
      await bearerClient(accessToken).auth.logout({ refresh_token: refreshToken });
    } catch {
      // Best-effort: an expired/invalid token still means the local session ends below.
    }
  }

  const res = NextResponse.json({ status: 'logged_out' }, { status: 200 });
  clearRefreshCookie(res);
  return res;
}
