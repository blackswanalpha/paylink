/**
 * POST /api/auth/password/reset-request — begin a password reset.
 *
 * Thin pass-through to identity-service, which is anti-enumeration: it ALWAYS returns 200 with the
 * same shape whether or not the account exists. The page shows an identical confirmation regardless,
 * so even a mapped transport error here never reveals whether an account exists. In dev the response
 * may carry `reset_token` (gated server-side by IDENTITY_PASSWORD_RESET_DEV_RETURN_TOKEN).
 */

import { NextResponse } from 'next/server';

import { anonClient } from '@/lib/authServer';
import { mapAuthError, type PasswordResetRequestBody } from '@/lib/authSession';

export const dynamic = 'force-dynamic';

export async function POST(req: Request): Promise<NextResponse> {
  let body: PasswordResetRequestBody;
  try {
    body = (await req.json()) as PasswordResetRequestBody;
  } catch {
    return NextResponse.json(
      { error: { code: 'INVALID_PAYLOAD', message: 'request body must be JSON', details: {} } },
      { status: 400 },
    );
  }

  try {
    const result = await anonClient().auth.requestPasswordReset({
      email: body.email,
      phone: body.phone,
    });
    return NextResponse.json(result, { status: 200 });
  } catch (err) {
    const { status, body: envelope } = mapAuthError(err);
    return NextResponse.json(envelope, { status });
  }
}
