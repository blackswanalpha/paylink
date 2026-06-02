'use client';

/**
 * PageTransition — wraps page content in a fade + small-rise enter, replayed on every navigation
 * (mounted by app/template.tsx, which the App Router re-mounts per route change). Enter-only:
 * App-Router page EXIT transitions need navigation interception and are out of scope (ADR-012).
 *
 * Reduced motion (F.6): when the OS requests it we render the final state immediately
 * (`initial={false}`) — no opacity OR transform transition, exactly as frontendfeature.md §2.4 asks.
 * (framer's MotionConfig only suppresses transforms, so the opacity fade is gated explicitly here.)
 */

import type { ReactNode } from 'react';
import { motion } from 'framer-motion';
import { useReducedMotion } from '@/hooks/useReducedMotion';
import { pageVariants } from './tokens';

export function PageTransition({ children }: { children: ReactNode }) {
  const reduced = useReducedMotion();
  return (
    <motion.div variants={pageVariants} initial={reduced ? false : 'hidden'} animate="show">
      {children}
    </motion.div>
  );
}
