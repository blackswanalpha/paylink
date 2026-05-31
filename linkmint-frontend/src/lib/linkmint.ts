/**
 * The one and only place the LinkMint SDK client is constructed — the app's single backend
 * surface. Every API call goes through this client (no raw `fetch` to the LinkMint API anywhere).
 */

import { LinkMintClient } from '@linkmint/sdk';
import { resolveApiBaseUrl } from './env';

/**
 * Build an SDK client authenticated with the server-minted dev JWT (bearer pass-through to the
 * gateway). `baseUrl` is the app's own origin, so `/v1/*` is same-origin and proxied to the
 * gateway by the Next rewrite. Uses the platform `fetch` (browser).
 */
export function createLinkMintClient(token: string): LinkMintClient {
  return new LinkMintClient({
    baseUrl: resolveApiBaseUrl(),
    auth: { type: 'bearer', token },
  });
}
