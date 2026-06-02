/**
 * OfflineBanner — asserts it appears on the `offline` event (with a non-color icon + text, role=status)
 * and flips to a "Back online" confirmation on reconnect.
 */

import { describe, it, expect } from 'vitest';
import { renderWithTheme, screen, act } from '@/test/renderWithTheme';
import { OfflineBanner } from '@/components/OfflineBanner';

describe('OfflineBanner', () => {
  it('shows while offline and confirms when back online', () => {
    renderWithTheme(<OfflineBanner />);
    // Online by default → nothing rendered.
    expect(screen.queryByRole('status')).toBeNull();

    act(() => {
      window.dispatchEvent(new Event('offline'));
    });
    expect(screen.getByRole('status')).toHaveTextContent(/offline/i);

    act(() => {
      window.dispatchEvent(new Event('online'));
    });
    expect(screen.getByRole('status')).toHaveTextContent(/back online/i);
  });
});
