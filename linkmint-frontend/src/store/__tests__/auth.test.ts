/**
 * useAuthStore — the in-memory identity session transitions used by the auth foundation.
 */

import { describe, it, expect, beforeEach } from 'vitest';
import type { UserProfile } from '@linkmint/sdk';

import { useAuthStore } from '@/store/auth';

const USER: UserProfile = {
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
};

beforeEach(() => {
  useAuthStore.setState({ status: 'unknown', user: null, accessToken: null, expiresAt: null });
});

describe('useAuthStore', () => {
  it('setSession establishes an authed session', () => {
    useAuthStore.getState().setSession({ accessToken: 't1', expiresAt: 123, user: USER });
    const s = useAuthStore.getState();
    expect(s.status).toBe('authed');
    expect(s.accessToken).toBe('t1');
    expect(s.user?.user_id).toBe('u1');
  });

  it('setToken swaps the token while keeping the profile', () => {
    useAuthStore.getState().setSession({ accessToken: 't1', expiresAt: 123, user: USER });
    useAuthStore.getState().setToken({ accessToken: 't2', expiresAt: 456 });
    const s = useAuthStore.getState();
    expect(s.accessToken).toBe('t2');
    expect(s.expiresAt).toBe(456);
    expect(s.user?.user_id).toBe('u1'); // unchanged
  });

  it('patchUser merges into the existing profile', () => {
    useAuthStore.getState().setSession({ accessToken: 't1', expiresAt: 123, user: USER });
    useAuthStore.getState().patchUser({ email: 'new@b.com' });
    expect(useAuthStore.getState().user?.email).toBe('new@b.com');
  });

  it('patchUser flips mfa_enabled without touching other fields', () => {
    useAuthStore.getState().setSession({ accessToken: 't1', expiresAt: 123, user: USER });
    useAuthStore.getState().patchUser({ mfa_enabled: true });
    const s = useAuthStore.getState();
    expect(s.user?.mfa_enabled).toBe(true);
    expect(s.user?.email).toBe('a@b.com'); // unchanged
  });

  it('clearSession drops to anonymous', () => {
    useAuthStore.getState().setSession({ accessToken: 't1', expiresAt: 123, user: USER });
    useAuthStore.getState().clearSession();
    const s = useAuthStore.getState();
    expect(s.status).toBe('anon');
    expect(s.user).toBeNull();
    expect(s.accessToken).toBeNull();
  });
});
