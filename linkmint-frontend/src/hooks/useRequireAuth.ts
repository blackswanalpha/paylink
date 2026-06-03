'use client';

/**
 * useRequireAuth — the client-side guard for protected pages. Bootstraps the session once when the
 * status is still `unknown` (cold load), and redirects to `/login?next=…` once it resolves to `anon`.
 * Returns `ready` (true only when `authed`) so the page can hold a skeleton until then.
 *
 * Client-side (not middleware) by design: the access token lives in memory, so only the browser can
 * decide whether the session is live — middleware sees only the opaque refresh cookie.
 */

import { useEffect } from 'react';
import { usePathname, useRouter } from 'next/navigation';

import { bootstrapSession } from '@/lib/authClient';
import { useAuthStore, type AuthStatus } from '@/store/auth';

export interface RequireAuthResult {
  /** True once the session is resolved AND authenticated — gate protected UI on this. */
  ready: boolean;
  status: AuthStatus;
}

export function useRequireAuth(): RequireAuthResult {
  const status = useAuthStore((s) => s.status);
  const router = useRouter();
  const pathname = usePathname();

  useEffect(() => {
    if (status === 'unknown') {
      void bootstrapSession();
    }
  }, [status]);

  useEffect(() => {
    if (status === 'anon') {
      const next = encodeURIComponent(pathname ?? '/');
      router.replace(`/login?next=${next}`);
    }
  }, [status, router, pathname]);

  return { ready: status === 'authed', status };
}
