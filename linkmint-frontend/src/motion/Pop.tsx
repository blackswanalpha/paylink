'use client';

/**
 * Pop — a brief scale-in for a just-changed inline element (e.g. the Copy→Check confirmation icon).
 * Replay it by changing the host element's `key` so this re-mounts. Transform-only, so the global
 * MotionConfig reducedMotion="user" (Provider.tsx) disables it under reduced motion (F.6). Motion is
 * never the ONLY signal (F.7) — pair it with the icon swap + toast that already exist.
 */

import type { ReactNode } from 'react';
import { motion } from 'framer-motion';
import { EASE } from './tokens';

export interface PopProps {
  children: ReactNode;
  /** When false, render with no entrance animation (the idle state). @default true */
  active?: boolean;
}

export function Pop({ children, active = true }: PopProps) {
  return (
    <motion.span
      initial={active ? { scale: 0.5 } : false}
      animate={{ scale: 1 }}
      transition={{ duration: 0.18, ease: EASE }}
      style={{ display: 'inline-flex' }}
    >
      {children}
    </motion.span>
  );
}
