/**
 * Server-only dev JWT minting.
 *
 * Mints a short-lived HS256 token the gateway accepts (an `iss`-bound credential plus the
 * `creator_addr` claim it injects downstream as `X-Creator-Addr`). The `import 'server-only'`
 * guard makes the build fail if this module is ever pulled into the client bundle, so the HMAC
 * secret never reaches the browser — only the signed token is handed to the client component.
 *
 * This mirrors the gateway's dev config in docker-compose.yml (HS256, issuer `linkmint-dev`,
 * secret `devsecret-change-me`, claim `creator_addr`). DEMO ONLY.
 */

import 'server-only';
import { createHmac } from 'node:crypto';

interface DevJwtConfig {
  secret: string;
  issuer: string;
  creatorAddr: string;
  ttlSeconds: number;
}

function base64url(input: string): string {
  return Buffer.from(input, 'utf8').toString('base64url');
}

function loadConfig(): DevJwtConfig {
  const secret = process.env.LINKMINT_JWT_DEV_SECRET;
  if (!secret) {
    throw new Error(
      'LINKMINT_JWT_DEV_SECRET is not set. Copy .env.example to .env.local (see the README).',
    );
  }
  const ttl = Number.parseInt(process.env.LINKMINT_JWT_TTL_SECONDS ?? '', 10);
  return {
    secret,
    issuer: process.env.LINKMINT_JWT_ISSUER || 'linkmint-dev',
    creatorAddr:
      process.env.LINKMINT_JWT_CREATOR_ADDR || '0x00000000000000000000000000000000000000bb',
    ttlSeconds: Number.isFinite(ttl) && ttl > 0 ? ttl : 3600,
  };
}

/**
 * The dev `creator_addr` the gateway injects as `X-Creator-Addr` — i.e. the address that owns the
 * PayLinks created through this app. Used server-side to scope the dashboard to "your" PayLinks.
 */
export function devCreatorAddr(): string {
  return loadConfig().creatorAddr;
}

/**
 * Mint a dev HS256 JWT. `now` (unix seconds) is injectable for tests; defaults to the wall clock.
 */
export function mintDevJwt(now: number = Math.floor(Date.now() / 1000)): string {
  const cfg = loadConfig();
  const header = { alg: 'HS256', typ: 'JWT' };
  const payload = {
    iss: cfg.issuer,
    creator_addr: cfg.creatorAddr,
    iat: now,
    nbf: now,
    exp: now + cfg.ttlSeconds,
  };
  const signingInput = `${base64url(JSON.stringify(header))}.${base64url(JSON.stringify(payload))}`;
  const signature = createHmac('sha256', cfg.secret).update(signingInput).digest('base64url');
  return `${signingInput}.${signature}`;
}
