'use client';

/**
 * useNotificationPreferences — the I/O for the Account → Notifications tab (work10). Loads the
 * caller's preferences from notification-service (`client.notifications.getPreferences`) and toggles
 * a single channel/event with an optimistic flip + a PUT patch. On failure it surfaces a toast (work04
 * `reportError`) and refetches to reconcile with the server — the same shape as `useNotifications`.
 *
 * Scoping note: these are keyed server-side by the creator address (X-Creator-Addr), so `client`
 * here must be the HS256/dashboard client — NOT the RS256 identity client the other Account tabs use.
 */

import { useCallback, useEffect, useState } from 'react';
import type {
  LinkMintClient,
  NotificationChannel,
  NotificationEventKind,
  NotificationPreferences,
} from '@linkmint/sdk';

import { isAbortError, type DisplayError } from '@/lib/errors';
import { reportError } from '@/lib/reportError';

export interface UseNotificationPreferencesResult {
  prefs: NotificationPreferences | null;
  loading: boolean;
  error: DisplayError | null;
  refresh: () => void;
  setChannel: (channel: NotificationChannel, value: boolean) => void;
  setEvent: (event: NotificationEventKind, value: boolean) => void;
}

export function useNotificationPreferences(
  client: LinkMintClient | null,
): UseNotificationPreferencesResult {
  const [prefs, setPrefs] = useState<NotificationPreferences | null>(null);
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
        const res = await client.notifications.getPreferences({ signal });
        if (!signal.aborted) {
          setPrefs(res);
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

  // Persist a single-key patch. The PUT returns the full effective set, which we adopt; on failure we
  // toast and refetch (the server is the source of truth — the optimistic flip is reconciled away).
  const persist = useCallback(
    (patch: Parameters<LinkMintClient['notifications']['updatePreferences']>[0]) => {
      if (!client) {
        return;
      }
      client.notifications
        .updatePreferences(patch)
        .then((updated) => setPrefs(updated))
        .catch((err: unknown) => {
          reportError(err, { surface: 'toast', context: 'while saving notification preferences' });
          refresh();
        });
    },
    [client, refresh],
  );

  const setChannel = useCallback(
    (channel: NotificationChannel, value: boolean) => {
      setPrefs((cur) => (cur ? { ...cur, channels: { ...cur.channels, [channel]: value } } : cur));
      persist({ channels: { [channel]: value } });
    },
    [persist],
  );

  const setEvent = useCallback(
    (event: NotificationEventKind, value: boolean) => {
      setPrefs((cur) => (cur ? { ...cur, events: { ...cur.events, [event]: value } } : cur));
      persist({ events: { [event]: value } });
    },
    [persist],
  );

  return { prefs, loading, error, refresh, setChannel, setEvent };
}
