/**
 * useRequireAuth — bootstraps the session when status is unknown and redirects to /login when anon;
 * reports ready only when authed.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';

import { useAuthStore } from '@/store/auth';

const { replaceMock, bootstrapMock } = vi.hoisted(() => ({
  replaceMock: vi.fn(),
  bootstrapMock: vi.fn().mockResolvedValue(undefined),
}));

vi.mock('next/navigation', () => ({
  useRouter: () => ({ replace: replaceMock, push: vi.fn() }),
  usePathname: () => '/account',
}));

vi.mock('@/lib/authClient', () => ({ bootstrapSession: bootstrapMock }));

import { useRequireAuth } from '@/hooks/useRequireAuth';

beforeEach(() => {
  replaceMock.mockClear();
  bootstrapMock.mockClear();
  useAuthStore.setState({ status: 'unknown', user: null, accessToken: null, expiresAt: null });
});

describe('useRequireAuth', () => {
  it('bootstraps the session when status is unknown', () => {
    renderHook(() => useRequireAuth());
    expect(bootstrapMock).toHaveBeenCalledTimes(1);
  });

  it('redirects to /login?next= when anonymous', async () => {
    useAuthStore.setState({ status: 'anon', user: null, accessToken: null, expiresAt: null });
    renderHook(() => useRequireAuth());
    await waitFor(() =>
      expect(replaceMock).toHaveBeenCalledWith(expect.stringContaining('/login?next=')),
    );
  });

  it('reports ready when authed', () => {
    useAuthStore.setState({
      status: 'authed',
      user: null,
      accessToken: 't',
      expiresAt: Date.now() + 60_000,
    });
    const { result } = renderHook(() => useRequireAuth());
    expect(result.current.ready).toBe(true);
    expect(replaceMock).not.toHaveBeenCalled();
  });
});
