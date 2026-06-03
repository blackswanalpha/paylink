'use client';

/**
 * useSessions — loads the caller's active sessions (`sessions.list`) and revokes one with an
 * optimistic removal (drop the row immediately, roll back + surface inline on error). Mirrors the
 * usePayLinks data/error idiom (work04/06): abortable load, 401 escalates to the global reauth modal.
 */

import { useCallback, useEffect, useState } from 'react';
import type { LinkMintClient, Session } from '@linkmint/sdk';

import { isAbortError, type DisplayError } from '@/lib/errors';
import { reportError } from '@/lib/reportError';

export interface UseSessionsResult {
  items: Session[];
  loading: boolean;
  error: DisplayError | null;
  refresh: () => void;
  /** Optimistically revoke (remove the row); rolls back + reports inline on failure. */
  revoke: (sessionId: string) => Promise<boolean>;
}

export function useSessions(client: LinkMintClient | null): UseSessionsResult {
  const [items, setItems] = useState<Session[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<DisplayError | null>(null);

  const load = useCallback(
    async (signal: AbortSignal) => {
      if (!client) {
        return;
      }
      setLoading(true);
      setError(null);
      try {
        const res = await client.sessions.list({ signal });
        if (!signal.aborted) {
          setItems(res.items);
        }
      } catch (err) {
        if (isAbortError(err) || signal.aborted) {
          return;
        }
        const { error: reported, surface } = reportError(err, { surface: 'inline' });
        if (surface === 'inline') {
          setError(reported);
        }
      } finally {
        if (!signal.aborted) {
          setLoading(false);
        }
      }
    },
    [client],
  );

  useEffect(() => {
    if (!client) {
      return;
    }
    const controller = new AbortController();
    void load(controller.signal);
    return () => controller.abort();
  }, [client, load]);

  const refresh = useCallback(() => {
    const controller = new AbortController();
    void load(controller.signal);
  }, [load]);

  const revoke = useCallback(
    async (sessionId: string): Promise<boolean> => {
      if (!client) {
        return false;
      }
      const removed = items.find((s) => s.session_id === sessionId);
      setItems((prev) => prev.filter((s) => s.session_id !== sessionId));
      try {
        await client.sessions.revoke(sessionId);
        return true;
      } catch (err) {
        // Functional rollback: re-insert only the removed row, preserving any concurrent changes.
        if (removed) {
          setItems((prev) =>
            prev.some((s) => s.session_id === sessionId) ? prev : [removed, ...prev],
          );
        }
        const { error: reported, surface } = reportError(err, { surface: 'inline' });
        if (surface === 'inline') {
          setError(reported);
        }
        return false;
      }
    },
    [client, items],
  );

  return { items, loading, error, refresh, revoke };
}
