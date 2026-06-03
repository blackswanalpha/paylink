/**
 * The one and only place the LinkMint SDK client is constructed — the app's single backend
 * surface. Every API call goes through this client (no raw `fetch` to the LinkMint API anywhere).
 *
 * Two token contexts, never mixed:
 *  - the HS256 dev token (a `string`) for the paylinks/payments demo (`/dashboard`); and
 *  - the RS256 identity session (a refreshing provider) for work09/10 (`createAuthedLinkMintClient`).
 * `baseUrl` is the app's own origin, so `/v1/*` is same-origin and proxied to the gateway by the
 * Next rewrite. Uses the platform `fetch` (browser).
 */

import { LinkMintClient } from '@linkmint/sdk';
import { getAccessToken } from './authClient';
import { resolveApiBaseUrl } from './env';

/** A bearer token: a fixed string, or a (possibly async) provider invoked per request for refresh. */
export type BearerToken = string | (() => string | Promise<string>);

/** Build an SDK client authenticated with the given bearer token (string or refreshing provider). */
export function createLinkMintClient(token: BearerToken): LinkMintClient {
  return new LinkMintClient({
    baseUrl: resolveApiBaseUrl(),
    auth: { type: 'bearer', token },
  });
}

/**
 * The identity-family client (auth/users/sessions/organizations, …) for work09/10 screens. It
 * authenticates with the RS256 session token from `useAuthStore`, transparently refreshed via
 * `getAccessToken`. Distinct from the HS256 dev-token client used by `/dashboard`.
 */
export function createAuthedLinkMintClient(): LinkMintClient {
  return createLinkMintClient(getAccessToken);
}
