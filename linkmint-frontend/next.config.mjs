/**
 * Next.js config for the LinkMint demo frontend.
 *
 * - `rewrites()` proxies the browser's same-origin `/v1/*` calls to the API gateway (Kong) so the
 *   SDK never makes a cross-origin request (the gateway has no CORS plugin). The SDK still performs
 *   every fetch; only the network hop is proxied by Next on the server side.
 * - `turbopack.root` is the repo root because the `@linkmint/sdk` dependency lives at
 *   `../sdks/javascript` (a sibling of this app); Turbopack must resolve modules above this dir.
 * - `allowedDevOrigins` lets the dev server serve its `/_next/*` dev resources (HMR, chunks) when the
 *   app is opened from a LAN IP (e.g. another device on the network), not just `localhost`. Next 16
 *   blocks cross-origin dev access by default, which silently breaks client hydration. We allow this
 *   machine's own LAN IPv4 addresses automatically so it survives DHCP changes.
 * - Linting is run separately via `npm run lint`; Next 16 no longer lints during `next build`.
 *
 * The local `@linkmint/sdk` ships prebuilt, browser-ready ESM, so no `transpilePackages` is needed.
 */

import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';
import { networkInterfaces } from 'node:os';

const projectDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(projectDir, '..');

const gatewayUrl = process.env.LINKMINT_GATEWAY_URL ?? 'http://localhost:8088';

// This machine's non-internal IPv4 addresses, so the dev server can be opened from another device.
const lanOrigins = Object.values(networkInterfaces())
  .flat()
  .filter((i) => i && i.family === 'IPv4' && !i.internal)
  .map((i) => i.address);

/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  turbopack: { root: repoRoot },
  allowedDevOrigins: lanOrigins,
  async rewrites() {
    return [{ source: '/v1/:path*', destination: `${gatewayUrl}/v1/:path*` }];
  },
};

export default nextConfig;
