'use client';

/**
 * useCountUp — animates an integer from `from` (default 0) up to `to` via framer-motion's `animate`
 * driver, easing on the Ivory standard curve. SSR-safe: state initializes to the FINAL value, so the
 * server and first client paint render the real number (never a fake 0) — F.7. Under reduced motion
 * it jumps straight to `to` (F.6). Values are rounded to integers (money is integer minor units).
 */

import { useEffect, useState } from 'react';
import { animate } from 'framer-motion';
import { useReducedMotion } from './useReducedMotion';
import { EASE } from '@/motion/tokens';

export interface UseCountUpOptions {
  /** Animation length in ms. @default 600 */
  durationMs?: number;
  /** Starting value. @default 0 */
  from?: number;
}

export function useCountUp(to: number, options?: UseCountUpOptions): number {
  const reduced = useReducedMotion();
  const durationMs = options?.durationMs ?? 600;
  const from = options?.from ?? 0;
  const [value, setValue] = useState(to);

  useEffect(() => {
    if (reduced) {
      setValue(to);
      return;
    }
    const controls = animate(from, to, {
      duration: durationMs / 1000,
      ease: EASE,
      onUpdate: (v) => setValue(Math.round(v)),
    });
    return () => controls.stop();
  }, [to, from, durationMs, reduced]);

  return value;
}
