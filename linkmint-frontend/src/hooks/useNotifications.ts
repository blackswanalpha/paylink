'use client';

/**
 * useNotifications — the I/O for the server-backed notification center (work07). Mirrors the
 * usePayLinks fetch shape: takes the SDK client, fetches via `client.notifications.*`, maps the wire
 * shape into the store, and reconciles optimistic read-flips on failure. Errors route through the
 * work04 error system (`reportError`) — the background fetch is silent (never hijacks a surface),
 * while a failed mark-read shows a single toast and refetches.
 */

import { useCallback, useState } from 'react';
import type { LinkMintClient, Notification as WireNotification } from '@linkmint/sdk';
import { isAbortError } from '@/lib/errors';
import { reportError } from '@/lib/reportError';
import { useNotificationStore, type AppNotification } from '@/store/notifications';

function toApp(n: WireNotification): AppNotification {
  const parsed = Date.parse(n.created_at);
  return {
    id: n.id,
    kind: n.kind,
    title: n.title,
    body: n.body,
    href: n.href,
    read: n.read,
    createdAt: Number.isNaN(parsed) ? 0 : parsed,
  };
}

export interface UseNotificationsResult {
  /** Fetch the latest inbox into the store. */
  refresh: () => void;
  /** Optimistically mark one read, then persist (reconcile on failure). */
  markRead: (id: string) => void;
  /** Optimistically mark all read, then persist (reconcile on failure). */
  markAllRead: () => void;
  loading: boolean;
}

export function useNotifications(client: LinkMintClient | null): UseNotificationsResult {
  const setItems = useNotificationStore((s) => s.setItems);
  const applyMarkRead = useNotificationStore((s) => s.applyMarkRead);
  const applyMarkAllRead = useNotificationStore((s) => s.applyMarkAllRead);
  const [loading, setLoading] = useState(false);

  const refresh = useCallback(() => {
    if (!client) return;
    const controller = new AbortController();
    setLoading(true);
    client.notifications
      .list({ limit: 50 }, { signal: controller.signal })
      .then((res) => setItems(res.items.map(toApp)))
      .catch((err: unknown) => {
        if (isAbortError(err)) return;
        // A background inbox poll must never hijack a surface (no reauth modal / toast from it):
        // normalize silently. The dashboard's primary fetch owns 401/visible errors.
        reportError(err, { silent: true });
      })
      .finally(() => setLoading(false));
  }, [client, setItems]);

  const markRead = useCallback(
    (id: string) => {
      if (!client) return;
      applyMarkRead(id);
      client.notifications.markRead(id).catch((err: unknown) => {
        reportError(err, { surface: 'toast', context: 'while updating a notification' });
        refresh();
      });
    },
    [client, applyMarkRead, refresh],
  );

  const markAllRead = useCallback(() => {
    if (!client) return;
    applyMarkAllRead();
    client.notifications.markAllRead().catch((err: unknown) => {
      reportError(err, { surface: 'toast', context: 'while updating notifications' });
      refresh();
    });
  }, [client, applyMarkAllRead, refresh]);

  return { refresh, markRead, markAllRead, loading };
}
