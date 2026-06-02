'use client';

/**
 * useRetry — the retry primitive for **idempotent reads only** (work04). The only thing it ever
 * re-runs is the caller's `run` fn, and only on an explicit `retry()` call — the error system never
 * re-fires the original failed request on its own, so a mutation can never be silently repeated. A
 * read screen passes its loader as `run`; a mutation screen simply never uses this hook.
 *
 * It also models the 429 `Retry-After` countdown: `noteError` (or a failed `run`) seeds `cooldown`
 * from the error's `retryAfter`, and `retry()` is a no-op until it elapses.
 */

import { useCallback, useEffect, useRef, useState } from 'react';
import type { DisplayError } from '@/lib/errors';
import { reportError } from '@/lib/reportError';

export interface UseRetryOptions<T> {
  /** The IDEMPOTENT READ to re-run. Never pass a state-mutating call here. */
  run: () => Promise<T>;
  /** Called with the fresh value on a successful retry. */
  onSuccess?: (value: T) => void;
  /** Called with the normalized error on failure. When omitted, the failure surfaces via the policy (toast for 5xx/transport). */
  onError?: (err: DisplayError) => void;
  /** Cap retries (the manual click counts as one). @default Infinity (user-driven). */
  maxAttempts?: number;
}

export interface UseRetryApi {
  /** Re-run `run`. No-op while a retry is in flight, during a 429 cooldown, or past `maxAttempts`. */
  retry: () => void;
  isRetrying: boolean;
  /** Seconds left before retry is allowed again (0 = ready now). */
  cooldown: number;
  /** Retries used so far. */
  attempts: number;
  /** Feed an error in so a 429 `Retry-After` starts the cooldown (for callers not using `run`'s catch). */
  noteError: (err: DisplayError) => void;
}

export function useRetry<T>({
  run,
  onSuccess,
  onError,
  maxAttempts = Infinity,
}: UseRetryOptions<T>): UseRetryApi {
  const [isRetrying, setIsRetrying] = useState(false);
  const [cooldown, setCooldown] = useState(0);
  const [attempts, setAttempts] = useState(0);

  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);

  // Keep the latest callbacks in refs so `retry`'s identity only depends on the guard state.
  const runRef = useRef(run);
  const onSuccessRef = useRef(onSuccess);
  const onErrorRef = useRef(onError);
  useEffect(() => {
    runRef.current = run;
    onSuccessRef.current = onSuccess;
    onErrorRef.current = onError;
  });

  // Tick the cooldown down once per second until it hits zero.
  useEffect(() => {
    if (cooldown <= 0) {
      return;
    }
    const id = setInterval(() => {
      setCooldown((c) => (c <= 1 ? 0 : c - 1));
    }, 1000);
    return () => clearInterval(id);
  }, [cooldown]);

  const noteError = useCallback((err: DisplayError) => {
    if (typeof err.retryAfter === 'number' && err.retryAfter > 0) {
      setCooldown(Math.ceil(err.retryAfter));
    }
  }, []);

  const retry = useCallback(() => {
    if (isRetrying || cooldown > 0 || attempts >= maxAttempts) {
      return;
    }
    setIsRetrying(true);
    setAttempts((a) => a + 1);
    runRef
      .current()
      .then((value) => {
        if (!mounted.current) return;
        setIsRetrying(false);
        onSuccessRef.current?.(value);
      })
      .catch((err: unknown) => {
        if (!mounted.current) return;
        setIsRetrying(false);
        // If the caller renders the error itself, normalize silently; otherwise let the policy toast.
        const { error } = reportError(err, onErrorRef.current ? { silent: true } : undefined);
        if (typeof error.retryAfter === 'number' && error.retryAfter > 0) {
          setCooldown(Math.ceil(error.retryAfter));
        }
        onErrorRef.current?.(error);
      });
  }, [isRetrying, cooldown, attempts, maxAttempts]);

  return { retry, isRetrying, cooldown, attempts, noteError };
}
