/**
 * Server-only SDK client factories for the `/api/auth/*` route handlers. They talk to the gateway
 * directly (server → gateway), not through the Next `/v1` rewrite, mirroring `lib/identity.ts`. The
 * refresh token and password never leave the server; only the access token is handed to the browser.
 */

import 'server-only';

import { LinkMintClient } from '@linkmint/sdk';

/** Server-side gateway base (same default as the Next rewrite in `next.config.mjs`). */
export function gatewayBaseUrl(): string {
  return process.env.LINKMINT_GATEWAY_URL ?? 'http://localhost:8088';
}

/** An unauthenticated client for the public auth endpoints (`register` / `login` / `refresh`). */
export function anonClient(): LinkMintClient {
  return new LinkMintClient({ baseUrl: gatewayBaseUrl() });
}

/** A bearer-authed client for endpoints that need the caller's token (`users.me`, `logout`). */
export function bearerClient(accessToken: string): LinkMintClient {
  return new LinkMintClient({
    baseUrl: gatewayBaseUrl(),
    auth: { type: 'bearer', token: accessToken },
  });
}
