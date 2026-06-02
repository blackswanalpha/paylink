'use client';

/**
 * Stagger / StaggerItem — entrance choreography for card grids and lists (work05 / ADR-012). Wrap a
 * group in <Stagger> and each child in <StaggerItem>; children fade + rise in sequence. Variant state
 * flows from the container to the items via framer's context, so an intermediate layout element
 * (a Chakra SimpleGrid/Stack) between them is fine.
 *
 * Reduced motion (F.6): renders the final state immediately — no opacity/transform transition.
 * Stagger is ENTRANCE-only — do not re-trigger it on an in-place data refresh (that reads as fake
 * loading, F.7); mount it once for the initial reveal.
 */

import type { ReactNode } from 'react';
import { motion } from 'framer-motion';
import { useReducedMotion } from '@/hooks/useReducedMotion';
import { listContainer, listItem } from './tokens';

export interface StaggerProps {
  children: ReactNode;
  /** Forwarded to the container element (layout helpers, etc.). */
  className?: string;
}

export function Stagger({ children, className }: StaggerProps) {
  const reduced = useReducedMotion();
  return (
    <motion.div
      className={className}
      variants={listContainer}
      initial={reduced ? false : 'hidden'}
      animate="show"
    >
      {children}
    </motion.div>
  );
}

export interface StaggerItemProps {
  children: ReactNode;
  className?: string;
}

export function StaggerItem({ children, className }: StaggerItemProps) {
  return (
    <motion.div className={className} variants={listItem}>
      {children}
    </motion.div>
  );
}
