/**
 * GlobalErrorOverlays — the 401/402 app-wide modals. Asserts the store drives each alertdialog with
 * its seam CTA. Phase-honesty: we assert the CTA is present and reachable, NOT that it navigates or
 * verifies (those are work09/work15 seams).
 */

import { describe, it, expect, beforeEach } from 'vitest';
import { renderWithTheme, screen, act } from '@/test/renderWithTheme';
import { GlobalErrorOverlays } from '@/components/GlobalErrorOverlays';
import { useErrorStore } from '@/store/errors';

beforeEach(() => {
  useErrorStore.setState({ reauth: null, kyc: null });
});

describe('GlobalErrorOverlays', () => {
  it('opens the re-auth alertdialog on a 401, with a "Sign in again" seam CTA', async () => {
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
