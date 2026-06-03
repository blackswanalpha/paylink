/**
 * useAuthStore (Zustand) — the in-memory identity session for work09/10. Holds the RS256 access
 * token, its expiry, and the authenticated profile. NOTHING is persisted (no localStorage): the
 * durable anchor is the httpOnly `lm_rt` refresh cookie, and `bootstrapSession` rehydrates from it on
 * cold load. `status` starts `unknown` until that probe resolves to `authed` or `anon`.
 *
 * This is SEPARATE from `store/app.ts` (which holds the HS256 dev-token paylinks client). The two
 * token contexts never mix — see `lib/linkmint.ts`.
 */

import { create } from 'zustand';
import type { UserProfile } from '@linkmint/sdk';

export type AuthStatus = 'unknown' | 'anon' | 'authed';

interface AuthState {
  status: AuthStatus;
  user: UserProfile | null;
  /** RS256 access token, in memory only. */
  accessToken: string | null;
  /** Access-token expiry as a ms epoch. */
  expiresAt: number | null;
  /** Establish a full session (login / bootstrap success). */
  setSession: (s: { accessToken: string; expiresAt: number; user: UserProfile }) => void;
  /** Swap in a refreshed access token, keeping the current profile (refresh hot path). */
  setToken: (t: { accessToken: string; expiresAt: number }) => void;
  /** Merge partial profile changes (e.g. after `users.updateMe`). */
  patchUser: (partial: Partial<UserProfile>) => void;
  /** Drop the session → anonymous (logout, or a dead refresh token). */
  clearSession: () => void;
}

export const useAuthStore = create<AuthState>((set) => ({
  status: 'unknown',
  user: null,
  accessToken: null,
  expiresAt: null,
  setSession: ({ accessToken, expiresAt, user }) =>
    set({ status: 'authed', accessToken, expiresAt, user }),
  setToken: ({ accessToken, expiresAt }) => set({ accessToken, expiresAt }),
  patchUser: (partial) => set((s) => (s.user ? { user: { ...s.user, ...partial } } : {})),
  clearSession: () => set({ status: 'anon', user: null, accessToken: null, expiresAt: null }),
}));
