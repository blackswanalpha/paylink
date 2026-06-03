/**
 * LoginForm — the MFA-correctness contract (F.5). A 401 `MFA_REQUIRED` must reveal the code field and
 * NEVER open the global reauth overlay (it would if the form didn't report silently + branch on code).
 * Also covers the full MFA round-trip and an inline invalid-credentials banner.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderWithTheme, screen, waitFor } from '@/test/renderWithTheme';
import { LoginForm } from '@/components/auth/LoginForm';
import { useErrorStore } from '@/store/errors';
import { useAuthStore } from '@/store/auth';

const { replaceMock } = vi.hoisted(() => ({ replaceMock: vi.fn() }));

vi.mock('next/navigation', () => ({
  useRouter: () => ({ replace: replaceMock, push: vi.fn() }),
  usePathname: () => '/login',
}));

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'content-type': 'application/json' },
  });
}

const SESSION_BODY = {
  accessToken: 'tok',
  expiresIn: 900,
  expiresAt: Date.now() + 900_000,
  user: {
    user_id: 'u1',
    email: 'a@b.com',
    phone: null,
    kyc_tier: 0,
    status: 'ACTIVE',
    mfa_enabled: false,
    roles: [],
    user_roles: [],
    created_at: '2026-01-01T00:00:00Z',
    last_login_at: null,
  },
};

beforeEach(() => {
  useErrorStore.setState({ reauth: null, kyc: null });
  useAuthStore.setState({ status: 'unknown', user: null, accessToken: null, expiresAt: null });
  replaceMock.mockClear();
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('LoginForm', () => {
  it('reveals the MFA field on MFA_REQUIRED WITHOUT opening the reauth overlay', async () => {
    vi.stubGlobal(
      'fetch',
      vi
        .fn()
        .mockResolvedValue(
          jsonResponse({ error: { code: 'MFA_REQUIRED', message: 'mfa required' } }, 401),
        ),
    );
    const { user } = renderWithTheme(<LoginForm />);

    await user.type(screen.getByLabelText(/email/i), 'a@b.com');
    await user.type(screen.getByLabelText(/password/i), 'password123');
    await user.click(screen.getByRole('button', { name: /^sign in$/i }));

    expect(await screen.findByLabelText(/authentication code/i)).toBeInTheDocument();
    // The critical assertion: a 401 here must NOT escalate to the global reauth overlay.
    expect(useErrorStore.getState().reauth).toBeNull();
  });

  it('completes login by submitting the MFA code', async () => {
    const fetchSpy = vi
      .fn()
      .mockResolvedValueOnce(
        jsonResponse({ error: { code: 'MFA_REQUIRED', message: 'mfa required' } }, 401),
      )
      .mockResolvedValueOnce(jsonResponse(SESSION_BODY));
    vi.stubGlobal('fetch', fetchSpy);
    const { user } = renderWithTheme(<LoginForm next="/account" />);

    await user.type(screen.getByLabelText(/email/i), 'a@b.com');
    await user.type(screen.getByLabelText(/password/i), 'password123');
    await user.click(screen.getByRole('button', { name: /^sign in$/i }));

    const codeField = await screen.findByLabelText(/authentication code/i);
    await user.type(codeField, '123456');
    await user.click(screen.getByRole('button', { name: /verify & sign in/i }));

    await waitFor(() => expect(replaceMock).toHaveBeenCalledWith('/account'));
    expect(useAuthStore.getState().status).toBe('authed');
    const secondCallBody = JSON.parse(String(fetchSpy.mock.calls[1]?.[1]?.body));
    expect(secondCallBody.mfa_code).toBe('123456');
  });

  it('shows an inline banner on invalid credentials (no overlay)', async () => {
    vi.stubGlobal(
      'fetch',
      vi
        .fn()
        .mockResolvedValue(
          jsonResponse({ error: { code: 'INVALID_CREDENTIALS', message: 'bad creds' } }, 401),
        ),
    );
    const { user } = renderWithTheme(<LoginForm />);

    await user.type(screen.getByLabelText(/email/i), 'a@b.com');
    await user.type(screen.getByLabelText(/password/i), 'wrong-pass');
    await user.click(screen.getByRole('button', { name: /^sign in$/i }));

    expect(await screen.findByText(/incorrect email or password/i)).toBeInTheDocument();
    expect(useErrorStore.getState().reauth).toBeNull();
  });
});
