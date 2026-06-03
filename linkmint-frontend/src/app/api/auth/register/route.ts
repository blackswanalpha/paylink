/**
 * POST /api/auth/register — create a new user.
 *
 * Decoupled from login: returns `{ user_id, status }` and the client then redirects to `/login`
 * (no auto-login, no cookie set here). Envelope errors (e.g. `EMAIL_TAKEN`) pass through unchanged.
 */

import { NextResponse } from 'next/server';

import { anonClient } from '@/lib/authServer';
import { mapAuthError, type RegisterRequestBody } from '@/lib/authSession';

export const dynamic = 'force-dynamic';

export async function POST(req: Request): Promise<NextResponse> {
  let body: RegisterRequestBody;
  try {
    body = (await req.json()) as RegisterRequestBody;
  } catch {
    return NextResponse.json(
      { error: { code: 'INVALID_PAYLOAD', message: 'request body must be JSON', details: {} } },
      { status: 400 },
    );
  }

  try {
    const result = await anonClient().auth.register({
      email: body.email,
      phone: body.phone,
      password: body.password,
    });
    return NextResponse.json(result, { status: 201 });
  } catch (err) {
    const { status, body: envelope } = mapAuthError(err);
    return NextResponse.json(envelope, { status });
  }
}
