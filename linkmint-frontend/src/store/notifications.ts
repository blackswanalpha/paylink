/**
 * Notification-center store (work07). A thin Zustand state container — the server-backed inbox
 * (notification-service via the SDK) is the source of truth; `setItems` replaces the list from a
 * fetch, and `applyMarkRead`/`applyMarkAllRead` are optimistic local flips the I/O hook reconciles.
 * No async lives here (mirrors `store/errors.ts`); per-call fetching is in `useNotifications`.
 */

import { create } from 'zustand';

export type NotificationKind = 'success' | 'info' | 'warning' | 'error';

/** A notification as held in the client store (wire `created_at` is parsed to a ms epoch). */
export interface AppNotification {
  id: string;
  kind: NotificationKind;
  title: string;
  body: string | null;
  /** Optional in-app deep link opened when the row is activated. */
  href: string | null;
  read: boolean;
  /** ms epoch, parsed from the wire `created_at`. */
  createdAt: number;
}

interface NotificationState {
  items: AppNotification[];
  /** Replace the inbox with the server's truth (mapped from the SDK list). */
  setItems: (items: AppNotification[]) => void;
  /** Optimistically flip one notification read (reconciled by the hook on failure). */
  applyMarkRead: (id: string) => void;
  /** Optimistically flip all notifications read. */
  applyMarkAllRead: () => void;
}

export const useNotificationStore = create<NotificationState>((set) => ({
  items: [],
  setItems: (items) => set({ items }),
  applyMarkRead: (id) =>
    set((s) => ({ items: s.items.map((n) => (n.id === id ? { ...n, read: true } : n)) })),
  applyMarkAllRead: () => set((s) => ({ items: s.items.map((n) => ({ ...n, read: true })) })),
}));

/** Unread count — `useNotificationStore(selectUnreadCount)`. */
export const selectUnreadCount = (s: NotificationState): number =>
  s.items.reduce((n, i) => (i.read ? n : n + 1), 0);
