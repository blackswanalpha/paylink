/**
 * Notification center (work07) — the bell (unread badge + aria-label, opens the panel), the panel
 * (list / mark-read / mark-all / empty state), and the live region (announces new arrivals only).
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { act } from 'react';
import { renderWithTheme, screen } from '@/test/renderWithTheme';
import { NotificationBell } from '@/components/notifications/NotificationBell';
import { NotificationCenter } from '@/components/notifications/NotificationCenter';
import { NotificationLiveRegion } from '@/components/notifications/NotificationLiveRegion';
import { useAppStore } from '@/store/app';
import { useNotificationStore, type AppNotification } from '@/store/notifications';

function n(over: Partial<AppNotification> = {}): AppNotification {
  return {
    id: 'a',
    kind: 'info',
    title: 'PayLink created',
    body: 'PayLink 0xpl is ready.',
    href: null,
    read: false,
    createdAt: Date.now(),
    ...over,
  };
}

beforeEach(() => {
  useNotificationStore.setState({ items: [] });
  useAppStore.setState({ client: null });
});

afterEach(() => {
  vi.clearAllMocks();
});

describe('NotificationBell', () => {
  it('puts the unread count on the aria-label and shows a badge', () => {
    useNotificationStore.setState({ items: [n({ id: 'a' }), n({ id: 'b', title: 'Settled' })] });
    renderWithTheme(<NotificationBell />);
    expect(screen.getByRole('button', { name: 'Notifications, 2 unread' })).toBeInTheDocument();
    expect(screen.getByText('2')).toBeInTheDocument();
  });

  it('labels plainly + hides the badge at zero unread', () => {
    useNotificationStore.setState({ items: [n({ id: 'a', read: true })] });
    renderWithTheme(<NotificationBell />);
    expect(screen.getByRole('button', { name: 'Notifications' })).toBeInTheDocument();
    expect(screen.queryByText('1')).not.toBeInTheDocument();
  });

  it('opens the panel on click and lists notifications', async () => {
    useNotificationStore.setState({ items: [n({ id: 'a' }), n({ id: 'b', title: 'Settled' })] });
    const { user } = renderWithTheme(<NotificationBell />);
    await user.click(screen.getByRole('button', { name: /Notifications/ }));
    expect(screen.getByRole('dialog', { name: 'Notifications' })).toBeInTheDocument();
    expect(screen.getByText('Settled')).toBeInTheDocument();
  });
});

describe('NotificationCenter', () => {
  const noop = (): void => {};

  it('renders the branded empty state when there are no notifications', () => {
    renderWithTheme(
      <NotificationCenter open onClose={noop} onMarkRead={noop} onMarkAllRead={noop} />,
    );
    expect(screen.getByText('No notifications yet')).toBeInTheDocument();
  });

  it('marks one read on row activation', async () => {
    useNotificationStore.setState({ items: [n({ id: 'a' }), n({ id: 'b', title: 'Settled' })] });
    const onMarkRead = vi.fn();
    const { user } = renderWithTheme(
      <NotificationCenter open onClose={noop} onMarkRead={onMarkRead} onMarkAllRead={noop} />,
    );
    await user.click(screen.getByRole('button', { name: /PayLink created/ }));
    expect(onMarkRead).toHaveBeenCalledWith('a');
  });

  it('marks all read; the action is disabled when nothing is unread', async () => {
    useNotificationStore.setState({ items: [n({ id: 'a' })] });
    const onMarkAllRead = vi.fn();
    const { user, rerender } = renderWithTheme(
      <NotificationCenter open onClose={noop} onMarkRead={noop} onMarkAllRead={onMarkAllRead} />,
    );
    await user.click(screen.getByRole('button', { name: 'Mark all as read' }));
    expect(onMarkAllRead).toHaveBeenCalledTimes(1);

    act(() => useNotificationStore.setState({ items: [n({ id: 'a', read: true })] }));
    rerender(
      <NotificationCenter open onClose={noop} onMarkRead={noop} onMarkAllRead={onMarkAllRead} />,
    );
    expect(screen.getByRole('button', { name: 'Mark all as read' })).toBeDisabled();
  });
});

describe('NotificationLiveRegion', () => {
  it('announces a new top notification but stays silent on the initial populated state', () => {
    renderWithTheme(<NotificationLiveRegion />);
    const region = screen.getByRole('status');
    expect(region).toHaveTextContent('');

    // First populated state seeds silently.
    act(() => useNotificationStore.setState({ items: [n({ id: 'a', title: 'First' })] }));
    expect(region).toHaveTextContent('');

    // A genuinely newer top item is announced.
    act(() =>
      useNotificationStore.setState({
        items: [n({ id: 'b', kind: 'success', title: 'Settled' }), n({ id: 'a', title: 'First' })],
      }),
    );
    expect(region).toHaveTextContent('success notification: Settled');
  });
});
