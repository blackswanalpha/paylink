/**
 * NotificationsTab (work10) — loads preferences from the SDK, renders channel + event switches
 * reflecting the server state, and persists a single-key patch (optimistically) on toggle.
 */

import { describe, it, expect, vi } from 'vitest';
import type { LinkMintClient, NotificationPreferences } from '@linkmint/sdk';
import { renderWithTheme, screen } from '@/test/renderWithTheme';
import { NotificationsTab } from '@/components/account/NotificationsTab';

function prefs(over: Partial<NotificationPreferences> = {}): NotificationPreferences {
  return {
    channels: { in_app: true, email: true, sms: false },
    events: {
      'paylink.created': true,
      'paylink.verified': true,
      'paylink.cancelled': true,
      'payment.failed': false,
    },
    updated_at: '2026-06-03T00:00:00Z',
    ...over,
  };
}

/** A LinkMintClient stub exposing just the notification-preferences methods this tab uses. */
function fakeClient(overrides: Partial<NotificationPreferences> = {}) {
  const getPreferences = vi.fn().mockResolvedValue(prefs(overrides));
  const updatePreferences = vi.fn().mockResolvedValue(prefs(overrides));
  const client = {
    notifications: { getPreferences, updatePreferences },
  } as unknown as LinkMintClient;
  return { client, getPreferences, updatePreferences };
}

describe('NotificationsTab', () => {
  it('renders channel + event switches reflecting the loaded preferences', async () => {
    const { client } = fakeClient();
    renderWithTheme(<NotificationsTab client={client} />);

    // Channels
    expect(await screen.findByRole('checkbox', { name: 'In-app' })).toBeChecked();
    expect(screen.getByRole('checkbox', { name: 'Email' })).toBeChecked();
    expect(screen.getByRole('checkbox', { name: 'SMS' })).not.toBeChecked();
    // Events
    expect(screen.getByRole('checkbox', { name: 'PayLink settled' })).toBeChecked();
    expect(screen.getByRole('checkbox', { name: 'Payment failed' })).not.toBeChecked();
  });

  it('persists a single-key patch when a channel is toggled off', async () => {
    const { client, updatePreferences } = fakeClient();
    const { user } = renderWithTheme(<NotificationsTab client={client} />);

    await user.click(await screen.findByText('Email'));

    expect(updatePreferences).toHaveBeenCalledWith({ channels: { email: false } });
  });

  it('persists a single-key patch when an event is toggled on', async () => {
    const { client, updatePreferences } = fakeClient();
    const { user } = renderWithTheme(<NotificationsTab client={client} />);

    // payment.failed starts false → toggling it sends { events: { 'payment.failed': true } }.
    await user.click(await screen.findByText('Payment failed'));

    expect(updatePreferences).toHaveBeenCalledWith({ events: { 'payment.failed': true } });
  });
});
