/**
 * POST /api/auth/password/reset-confirm — complete a password reset with a reset token.
 *
 * Pass-through to identity-service, which sets the new password and revokes ALL of the user's
 * sessions. No cookie work here: the user isn't logged in, and any prior session was just killed
 * server-side. A bad/expired/used token surfaces as a 401 `INVALID_TOKEN` envelope.
 */

import { NextResponse } from 'next/server';

import { anonClient } from '@/lib/authServer';
import { mapAuthError, type PasswordResetConfirmBody } from '@/lib/authSession';

export const dynamic = 'force-dynamic';

export async function POST(req: Request): Promise<NextResponse> {
  let body: PasswordResetConfirmBody;
  try {
    body = (await req.json()) as PasswordResetConfirmBody;
  } catch {
    return NextResponse.json(
      { error: { code: 'INVALID_PAYLOAD', message: 'request body must be JSON', details: {} } },
      { status: 400 },
    );
  }

  try {
    const result = await anonClient().auth.confirmPasswordReset({
      token: body.token,
      new_password: body.new_password,
    });
    return NextResponse.json(result, { status: 200 });
  } catch (err) {
    const { status, body: envelope } = mapAuthError(err);
    return NextResponse.json(envelope, { status });
  }
}
