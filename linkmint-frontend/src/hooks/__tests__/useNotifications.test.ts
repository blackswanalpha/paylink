/**
 * useNotifications (work07) — fetch maps the wire shape into the store, mark-read flips optimistically
 * then persists via the SDK, and everything no-ops without a client.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import type { LinkMintClient } from '@linkmint/sdk';
import { renderHook, act, waitFor } from '@/test/renderWithTheme';
import { useNotifications } from '@/hooks/useNotifications';
import { useNotificationStore, type AppNotification } from '@/store/notifications';

function wire(over: Record<string, unknown> = {}) {
  return {
    id: 'a',
    kind: 'success',
    title: 'PayLink settled',
    body: 'verified on-chain',
    href: '/dashboard/paylinks',
    read: false,
    created_at: '2026-06-02T00:00:00Z',
    ...over,
  };
}

function fakeClient() {
  const list = vi.fn().mockResolvedValue({ items: [wire()], next_cursor: null });
  const markRead = vi.fn().mockResolvedValue(wire({ read: true }));
  const markAllRead = vi.fn().mockResolvedValue({ count: 1 });
  const client = { notifications: { list, markRead, markAllRead } } as unknown as LinkMintClient;
  return { client, list, markRead, markAllRead };
}

const local = (over: Partial<AppNotification> = {}): AppNotification => ({
  id: 'a',
  kind: 'info',
  title: 'T',
  body: null,
  href: null,
  read: false,
  createdAt: 1,
  ...over,
});

beforeEach(() => {
  useNotificationStore.setState({ items: [] });
});

describe('useNotifications', () => {
  it('refresh maps the wire list into the store (created_at → ms)', async () => {
    const { client } = fakeClient();
    const { result } = renderHook(() => useNotifications(client));
    act(() => result.current.refresh());
    await waitFor(() => expect(useNotificationStore.getState().items).toHaveLength(1));
    const items = useNotificationStore.getState().items;
    expect(items[0]?.kind).toBe('success');
    expect(items[0]?.createdAt).toBe(Date.parse('2026-06-02T00:00:00Z'));
  });

  it('markRead flips optimistically, then calls the SDK', async () => {
    useNotificationStore.setState({ items: [local({ id: 'a' })] });
    const { client, markRead } = fakeClient();
    const { result } = renderHook(() => useNotifications(client));
    act(() => result.current.markRead('a'));
    expect(useNotificationStore.getState().items[0]?.read).toBe(true); // optimistic
    await waitFor(() => expect(markRead).toHaveBeenCalledWith('a'));
  });

  it('no-ops without a client', () => {
    const { result } = renderHook(() => useNotifications(null));
    act(() => result.current.refresh());
    expect(useNotificationStore.getState().items).toEqual([]);
  });
});
