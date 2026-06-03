/**
 * Server-only identity helper (work08 SDK-expansion smoke). The identity-family services verify the
 * RS256 token identity-service issues — the dashboard's HS256 dev token is NOT accepted by them — so
 * to read anything we must first mint a real token via `auth.login`. This logs in a fixed dev user
 * (registering it once if needed) and returns their profile via the new `users.me` SDK method.
 *
 * It talks to the gateway directly (server → gateway), not through the Next `/v1` rewrite, and the
 * password never leaves the server. This previews a sliver of the Account screen (work10) purely to
 * prove the new SDK→gateway→identity path end-to-end; the real auth/account UI is work09/10.
 */

import 'server-only';

import { isLinkMintApiError, LinkMintClient, type UserProfile } from '@linkmint/sdk';

/** Server-side gateway base (same default as the Next rewrite in next.config.mjs). */
function gatewayUrl(): string {
  return process.env.LINKMINT_GATEWAY_URL ?? 'http://localhost:8088';
}

function devUser(): { email: string; password: string } {
  return {
    email: process.env.LINKMINT_DEV_USER_EMAIL ?? 'dev+work08@linkmint.local',
    password: process.env.LINKMINT_DEV_USER_PASSWORD ?? 'work08-dev-password',
  };
}

/**
 * Register-or-login the dev user against identity-service, then read the profile with the issued
 * RS256 token. Throws (for the caller to render) if identity is unreachable or login fails.
 */
export async function getDevIdentitySession(): Promise<{ profile: UserProfile }> {
  const baseUrl = gatewayUrl();
  const { email, password } = devUser();

  const anon = new LinkMintClient({ baseUrl });

  // Best-effort register — on a repeat run the user already exists (a 4xx envelope), which is fine;
  // login is the source of truth. A transport error here would also fail login, so it surfaces there.
  try {
    await anon.auth.register({ email, password });
  } catch (err) {
    if (!isLinkMintApiError(err)) {
      throw err;
    }
  }

  const tokens = await anon.auth.login({ email, password });
  const authed = new LinkMintClient({
    baseUrl,
    auth: { type: 'bearer', token: tokens.access_token },
  });
  const profile = await authed.users.me();
  return { profile };
}
