/**
 * POST /api/auth/login — exchange credentials for a session.
 *
 * Calls identity-service `auth.login` server-side, stores the opaque refresh token in the httpOnly
 * `lm_rt` cookie, and returns the access token + profile to the browser. On an SDK error the upstream
 * status + envelope are passed through UNCHANGED so the client can read `code` — notably the 401
 * `MFA_REQUIRED` that drives the MFA challenge (see `authClient`/`LoginForm`).
 */

import { NextResponse } from 'next/server';

import { setRefreshCookie } from '@/lib/authCookies';
import { anonClient, bearerClient } from '@/lib/authServer';
import { mapAuthError, type LoginRequestBody, type SessionPayload } from '@/lib/authSession';

export const dynamic = 'force-dynamic';

export async function POST(req: Request): Promise<NextResponse> {
  let body: LoginRequestBody;
  try {
    body = (await req.json()) as LoginRequestBody;
  } catch {
    return NextResponse.json(
      { error: { code: 'INVALID_PAYLOAD', message: 'request body must be JSON', details: {} } },
      { status: 400 },
    );
  }

  try {
    const tokens = await anonClient().auth.login({
      email: body.email,
      phone: body.phone,
      password: body.password,
      mfa_code: body.mfa_code,
    });
    // Hydrate the profile with the freshly-minted token so the client has it in one round trip.
    const user = await bearerClient(tokens.access_token).users.me();

    const payload: SessionPayload = {
      accessToken: tokens.access_token,
      expiresIn: tokens.expires_in,
      expiresAt: Date.now() + tokens.expires_in * 1000,
      user,
    };
    const res = NextResponse.json(payload, { status: 200 });
    setRefreshCookie(res, tokens.refresh_token);
    return res;
  } catch (err) {
    const { status, body: envelope } = mapAuthError(err);
    return NextResponse.json(envelope, { status });
  }
}
