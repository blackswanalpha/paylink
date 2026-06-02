/**
 * Notification store (work07) — the state container's reducers + the unread-count selector.
 */

import { describe, it, expect, beforeEach } from 'vitest';
import {
  selectUnreadCount,
  useNotificationStore,
  type AppNotification,
} from '@/store/notifications';

function n(over: Partial<AppNotification> = {}): AppNotification {
  return {
    id: 'a',
    kind: 'info',
    title: 'T',
    body: null,
    href: null,
    read: false,
    createdAt: 1,
    ...over,
  };
}

beforeEach(() => {
  useNotificationStore.setState({ items: [] });
});

describe('notification store', () => {
  it('setItems replaces the list', () => {
    useNotificationStore.getState().setItems([n({ id: 'a' }), n({ id: 'b' })]);
    expect(useNotificationStore.getState().items).toHaveLength(2);
  });

  it('selectUnreadCount counts only unread items', () => {
    useNotificationStore
      .getState()
      .setItems([n({ id: 'a' }), n({ id: 'b', read: true }), n({ id: 'c' })]);
    expect(selectUnreadCount(useNotificationStore.getState())).toBe(2);
  });

  it('applyMarkRead flips exactly one notification read', () => {
    useNotificationStore.getState().setItems([n({ id: 'a' }), n({ id: 'b' })]);
    useNotificationStore.getState().applyMarkRead('a');
    const { items } = useNotificationStore.getState();
    expect(items.find((i) => i.id === 'a')?.read).toBe(true);
    expect(items.find((i) => i.id === 'b')?.read).toBe(false);
  });

  it('applyMarkAllRead flips every notification read', () => {
    useNotificationStore.getState().setItems([n({ id: 'a' }), n({ id: 'b' })]);
    useNotificationStore.getState().applyMarkAllRead();
    expect(selectUnreadCount(useNotificationStore.getState())).toBe(0);
  });
});
