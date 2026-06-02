/**
 * useErrorHandler — the declarative wrapper. Asserts inline-surfaced errors are stashed (and clearable)
 * while toast-surfaced errors leave `inlineError` null, and that per-call opts override the defaults.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, act } from '@/test/renderWithTheme';
import { createApiError, type ApiErrorInit } from '@linkmint/sdk';
import { useErrorHandler } from '@/hooks/useErrorHandler';
import { useErrorStore } from '@/store/errors';

vi.mock('sonner', () => ({ toast: { error: vi.fn() } }));

function api(status: number, code: string, extra: Partial<ApiErrorInit> = {}): Error {
  return createApiError({
    status,
    code,
    message: 'm',
    details: {},
    traceId: undefined,
    requestId: undefined,
    ...extra,
  });
}

beforeEach(() => {
  useErrorStore.setState({ reauth: null, kyc: null });
});

describe('useErrorHandler', () => {
  it('stashes an inline error and clears it', () => {
    const { result } = renderHook(() => useErrorHandler({ surface: 'inline' }));

    act(() => {
      result.current.report(api(404, 'PAYLINK_NOT_FOUND'));
    });
    expect(result.current.inlineError).not.toBeNull();

    act(() => {
      result.current.clear();
    });
    expect(result.current.inlineError).toBeNull();
  });

  it('leaves inlineError null for a toast-surfaced error', () => {
    const { result } = renderHook(() => useErrorHandler());

    act(() => {
      result.current.report(api(500, 'INTERNAL_ERROR'));
    });
    expect(result.current.inlineError).toBeNull();
  });

  it('per-call opts override the hook defaults', () => {
    const { result } = renderHook(() => useErrorHandler({ surface: 'toast' }));

    act(() => {
      result.current.report(api(500, 'INTERNAL_ERROR'), { surface: 'inline' });
    });
    expect(result.current.inlineError).not.toBeNull();
  });
});
