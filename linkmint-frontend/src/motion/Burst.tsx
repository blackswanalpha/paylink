'use client';

/**
 * Burst — a one-shot champagne "success" pop for a celebratory moment (a settlement verifying).
 * Renders its children; when `play` flips true it plays a brief scale + champagne ring pulse. The DOM
 * is stable whether or not it plays (no remount/flicker).
 *
 * Reduced motion (F.6): no animation — children render unchanged. Motion is never the ONLY signal
 * (F.7) — always pair Burst with text/icon/toast.
 */

import type { ReactNode } from 'react';
import { motion } from 'framer-motion';
import { useReducedMotion } from '@/hooks/useReducedMotion';
import { DURATION, EASE } from './tokens';

export interface BurstProps {
  /** Flip to true on the celebratory transition (e.g. status becomes VERIFIED). */
  play: boolean;
  children: ReactNode;
}

export function Burst({ play, children }: BurstProps) {
  const reduced = useReducedMotion();
  return (
    <motion.div
      animate={
        play && !reduced
          ? {
              scale: [0.96, 1.02, 1],
              boxShadow: [
                '0 0 0 0 rgba(200, 162, 75, 0)',
                '0 0 0 8px rgba(200, 162, 75, 0.35)',
                '0 0 0 0 rgba(200, 162, 75, 0)',
              ],
            }
          : { scale: 1 }
      }
      transition={{ duration: DURATION.slow, ease: EASE }}
      style={{ borderRadius: 8 }}
    >
      {children}
    </motion.div>
  );
}
