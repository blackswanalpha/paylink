/**
 * GlobalErrorOverlays — the 401/402 app-wide modals. Asserts the store drives each alertdialog and
 * that the re-auth CTA now (work09) clears the identity session and routes to /login, while the KYC
 * CTA is still a work15 seam (present + reachable, doesn't verify).
 */

import { describe, it, expect, beforeEach, vi } from 'vitest';
import { renderWithTheme, screen, act } from '@/test/renderWithTheme';
import { GlobalErrorOverlays } from '@/components/GlobalErrorOverlays';
import { useErrorStore } from '@/store/errors';
import { useAuthStore } from '@/store/auth';

const { pushMock } = vi.hoisted(() => ({ pushMock: vi.fn() }));

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: pushMock, replace: vi.fn(), prefetch: vi.fn() }),
  usePathname: () => '/account',
}));

beforeEach(() => {
  useErrorStore.setState({ reauth: null, kyc: null });
  useAuthStore.setState({
    status: 'authed',
    user: null,
    accessToken: 'tok',
    expiresAt: Date.now() + 60_000,
  });
  pushMock.mockClear();
});

describe('GlobalErrorOverlays', () => {
  it('opens the re-auth alertdialog on a 401, with a "Sign in again" CTA', async () => {
    renderWithTheme(<GlobalErrorOverlays />);
    act(() => {
      useErrorStore.getState().requireReauth({
        kind: 'api',
        title: 'Authentication failed',
        message: 'Your session has expired.',
        status: 401,
      });
    });

    expect(await screen.findByRole('alertdialog', { name: 'Session expired' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Sign in again' })).toBeInTheDocument();
  });

  it('the re-auth CTA clears the session and routes to /login', async () => {
    const { user } = renderWithTheme(<GlobalErrorOverlays />);
    act(() => {
      useErrorStore.getState().requireReauth({
        kind: 'api',
        title: 'Authentication failed',
        message: 'Your session has expired.',
        status: 401,
      });
    });
    await screen.findByRole('alertdialog', { name: 'Session expired' });

    await user.click(screen.getByRole('button', { name: 'Sign in again' }));

    expect(useAuthStore.getState().status).toBe('anon');
    expect(useAuthStore.getState().accessToken).toBeNull();
    expect(useErrorStore.getState().reauth).toBeNull();
    expect(pushMock).toHaveBeenCalledWith(expect.stringContaining('/login'));
  });

  it('opens the KYC alertdialog on a 402, with a "Verify identity" seam CTA', async () => {
    renderWithTheme(<GlobalErrorOverlays />);
    act(() => {
      useErrorStore.getState().requireKyc({
        kind: 'api',
        title: 'Verification required',
        message: 'Verify your identity to continue.',
        status: 402,
        code: 'KYC_REQUIRED',
      });
    });

    expect(
      await screen.findByRole('alertdialog', { name: 'Verification required' }),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Verify identity' })).toBeInTheDocument();
  });
});
