/**
 * useRetry — the idempotent-reads-only retry primitive. Asserts it runs the caller's `run` (and only
 * that), is single-flight while in progress, and that a 429 Retry-After gates the next attempt until
 * the cooldown elapses. The "reads only" guarantee is structural: the hook never re-fires anything but
 * the `run` it was handed.
 */

import { describe, it, expect, vi } from 'vitest';
import { renderHook, act } from '@/test/renderWithTheme';
import { useRetry } from '@/hooks/useRetry';
import type { DisplayError } from '@/lib/errors';

describe('useRetry', () => {
  it('runs the read and fires onSuccess with the value', async () => {
    const run = vi.fn().mockResolvedValue('ok');
    const onSuccess = vi.fn();
    const { result } = renderHook(() => useRetry({ run, onSuccess }));

    await act(async () => {
      result.current.retry();
    });

    expect(run).toHaveBeenCalledTimes(1);
    expect(onSuccess).toHaveBeenCalledWith('ok');
  });

  it('is single-flight: a second retry while in progress is a no-op', async () => {
    let resolve: (v: string) => void = () => undefined;
    const run = vi.fn().mockImplementation(() => new Promise<string>((r) => (resolve = r)));
    const { result } = renderHook(() => useRetry({ run }));

    act(() => {
      result.current.retry();
    });
    act(() => {
      result.current.retry(); // ignored — still in flight
    });
    expect(run).toHaveBeenCalledTimes(1);

    await act(async () => {
      resolve('done');
    });
  });

  it('a 429 Retry-After gates retry until the cooldown elapses', async () => {
    vi.useFakeTimers();
    try {
      const run = vi.fn().mockResolvedValue('ok');
      const { result } = renderHook(() => useRetry({ run }));

      const rateLimited: DisplayError = {
        kind: 'api',
        title: 'Rate limited',
        message: 'm',
        status: 429,
        retryAfter: 3,
      };
      act(() => {
        result.current.noteError(rateLimited);
      });
      expect(result.current.cooldown).toBe(3);

      act(() => {
        result.current.retry(); // blocked by the cooldown
      });
      expect(run).not.toHaveBeenCalled();

      act(() => {
        vi.advanceTimersByTime(3000);
      });
      expect(result.current.cooldown).toBe(0);

      await act(async () => {
        result.current.retry();
      });
      expect(run).toHaveBeenCalledTimes(1);
    } finally {
      vi.useRealTimers();
    }
  });
});
