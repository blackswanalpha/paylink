/**
 * Shared auth-session contract between the browser (`lib/authClient.ts`) and the Next route handlers
 * (`app/api/auth/*`). It carries the request/response shapes plus `mapAuthError`, which converts a
 * value thrown by a server-side SDK call back into an HTTP status + LinkMint error envelope. The
 * browser then reconstructs that envelope into a real `LinkMintApiError` (see `authClient`), so the
 * error classifies identically to a direct SDK call — crucial because login signals MFA as a 401
 * with `code: "MFA_REQUIRED"` and the login form must read `code`, not just the status.
 *
 * Plain module (no `server-only`/`use client`): the types are imported on both sides; `mapAuthError`
 * is pure and only called server-side.
 */

import { isLinkMintApiError, type ErrorEnvelope, type UserProfile } from '@linkmint/sdk';

/** Name of the httpOnly cookie holding the opaque, single-use refresh token. */
export const REFRESH_COOKIE = 'lm_rt';

/** Token half of a session — returned by `POST /api/auth/refresh` (the hot path; no profile). */
export interface RefreshPayload {
  /** RS256 access token; held in memory on the client, sent as `Authorization: Bearer`. */
  accessToken: string;
  /** Access-token lifetime in seconds (from the identity `TokenResponse`). */
  expiresIn: number;
  /** Absolute expiry as a ms epoch — computed server-side as `Date.now() + expiresIn * 1000`. */
  expiresAt: number;
}

/** Full session — returned by `POST /api/auth/login` (token + hydrated profile). */
export interface SessionPayload extends RefreshPayload {
  user: UserProfile;
}

/** Response of `GET /api/auth/session` — the cold-load bootstrap probe (always HTTP 200). */
export type SessionProbe = ({ authenticated: true } & SessionPayload) | { authenticated: false };

/** Body of `POST /api/auth/login`. Provide exactly one of `email`/`phone`. */
export interface LoginRequestBody {
  email?: string;
  phone?: string;
  password: string;
  /** TOTP code; only sent on the second submit once the first returns `MFA_REQUIRED`. */
  mfa_code?: string;
}

/** Body of `POST /api/auth/register`. Provide exactly one of `email`/`phone`. */
export interface RegisterRequestBody {
  email?: string;
  phone?: string;
  password: string;
}

/** Body of `POST /api/auth/password/reset-request`. Provide exactly one of `email`/`phone`. */
export interface PasswordResetRequestBody {
  email?: string;
  phone?: string;
}

/** Body of `POST /api/auth/password/reset-confirm`. */
export interface PasswordResetConfirmBody {
  token: string;
  new_password: string;
}

/** Map a value thrown by a server-side SDK call to an HTTP status + LinkMint error envelope. */
export function mapAuthError(err: unknown): { status: number; body: ErrorEnvelope } {
  if (isLinkMintApiError(err)) {
    return {
      status: err.status,
      body: {
        error: {
          code: err.code,
          message: err.message,
          details: err.details,
          trace_id: err.traceId,
        },
      },
    };
  }
  // Transport / unexpected — identity-service (or the gateway in front of it) is unreachable.
  return {
    status: 502,
    body: {
      error: {
        code: 'BAD_GATEWAY',
        message: 'the identity service could not be reached',
        details: {},
      },
    },
  };
}
