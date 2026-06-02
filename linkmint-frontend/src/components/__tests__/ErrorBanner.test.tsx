/**
 * ErrorBanner — the inline surface. Asserts the envelope renders, the trace id is copyable, the error
 * is announced via aria-live, and the retry affordance is gated correctly (present only when a read
 * supplies onRetry; disabled + labelled during a 429 cooldown).
 */

import { describe, it, expect, vi } from 'vitest';
import { renderWithTheme, screen } from '@/test/renderWithTheme';
import { ErrorBanner } from '@/components/ErrorBanner';
import type { DisplayError } from '@/lib/errors';

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const base: DisplayError = {
  kind: 'api',
  title: 'Rate limited',
  message: 'Slow down.',
  status: 429,
  code: 'RATE_LIMITED',
};

describe('ErrorBanner', () => {
  it('renders the envelope (title, message, code · status)', () => {
    renderWithTheme(<ErrorBanner error={base} />);
    expect(screen.getByText('Rate limited')).toBeInTheDocument();
    expect(screen.getByText('Slow down.')).toBeInTheDocument();
    expect(screen.getByText('RATE_LIMITED')).toBeInTheDocument();
  });

  it('announces errors assertively and transport failures politely (aria-live)', () => {
    const { container, rerender } = renderWithTheme(<ErrorBanner error={base} />);
    expect(container.querySelector('[aria-live="assertive"]')).not.toBeNull();

    rerender(<ErrorBanner error={{ kind: 'transport', title: 'Offline', message: 'x' }} />);
    expect(container.querySelector('[aria-live="polite"]')).not.toBeNull();
  });

  it('copies the trace id to the clipboard', async () => {
    const writeText = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue(undefined);
    const { user } = renderWithTheme(<ErrorBanner error={{ ...base, traceId: 'trace-xyz' }} />);
    await user.click(screen.getByRole('button', { name: 'Copy trace id' }));
    expect(writeText).toHaveBeenCalledWith('trace-xyz');
  });

  it('shows a retry button only when onRetry is supplied (mutations get none)', () => {
    const { rerender } = renderWithTheme(<ErrorBanner error={base} />);
    expect(screen.queryByRole('button', { name: /try again/i })).toBeNull();

    const onRetry = vi.fn();
    rerender(<ErrorBanner error={base} onRetry={onRetry} />);
    expect(screen.getByRole('button', { name: 'Try again' })).toBeInTheDocument();
  });

  it('disables retry during a cooldown and shows the countdown label', () => {
    renderWithTheme(<ErrorBanner error={base} onRetry={() => undefined} retryCooldown={5} />);
    expect(screen.getByRole('button', { name: 'Try again in 5s' })).toBeDisabled();
  });

  it('renders a custom action CTA', () => {
    renderWithTheme(<ErrorBanner error={base} action={<button type="button">Go home</button>} />);
    expect(screen.getByRole('button', { name: 'Go home' })).toBeInTheDocument();
  });
});
