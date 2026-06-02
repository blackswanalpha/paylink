/**
 * reportError — the routing layer. Asserts each surface dispatches to the right place: toast →
 * `notify.error` (work07), reauth/kyc → the overlay store, inline/page → nothing (caller renders).
 * Also the load-bearing rules: a `forced` 401 ignores a caller's downgrade hint, and `silent`
 * dispatches nothing. Mocking `@/lib/notify` (not Sonner) is the F.5 single-surface regression guard.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { createApiError, type ApiErrorInit } from '@linkmint/sdk';
import { notify } from '@/lib/notify';
import { reportError } from '@/lib/reportError';
import { useErrorStore } from '@/store/errors';

vi.mock('@/lib/notify', () => ({ notify: { error: vi.fn() } }));

function api(status: number, code = 'X', message = 'm', extra: Partial<ApiErrorInit> = {}): Error {
  return createApiError({
    status,
    code,
    message,
    details: {},
    traceId: undefined,
    requestId: undefined,
    ...extra,
  });
}

beforeEach(() => {
  vi.clearAllMocks();
  useErrorStore.setState({ reauth: null, kyc: null });
});

describe('reportError', () => {
  it('5xx → toast', () => {
    const { surface } = reportError(api(500, 'INTERNAL_ERROR'));
    expect(surface).toBe('toast');
    expect(notify.error).toHaveBeenCalledTimes(1);
  });

  it('5xx with surface:inline → no toast (caller renders)', () => {
    const { surface } = reportError(api(500, 'INTERNAL_ERROR'), { surface: 'inline' });
    expect(surface).toBe('inline');
    expect(notify.error).not.toHaveBeenCalled();
  });

  it('401 cannot be downgraded → dispatches re-auth, never toasts', () => {
    const { surface } = reportError(api(401, 'UNAUTHORIZED'), { surface: 'toast' });
    expect(surface).toBe('reauth');
    expect(useErrorStore.getState().reauth).not.toBeNull();
    expect(notify.error).not.toHaveBeenCalled();
  });

  it('402 → dispatches the KYC overlay', () => {
    reportError(api(402, 'KYC_REQUIRED'));
    expect(useErrorStore.getState().kyc).not.toBeNull();
  });

  it('402 with surface:inline → no store dispatch (inline gate instead)', () => {
    const { surface } = reportError(api(402, 'KYC_REQUIRED'), { surface: 'inline' });
    expect(surface).toBe('inline');
    expect(useErrorStore.getState().kyc).toBeNull();
  });

  it('silent → no toast, no store dispatch, still returns the normalized error', () => {
    const { error } = reportError(api(500, 'INTERNAL_ERROR'), { silent: true });
    expect(notify.error).not.toHaveBeenCalled();
    expect(useErrorStore.getState().reauth).toBeNull();
    expect(error.status).toBe(500);
  });
});
