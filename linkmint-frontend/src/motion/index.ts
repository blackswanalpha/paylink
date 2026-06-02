/**
 * The motion system (work05 / ADR-012) — one import surface for animation primitives. framer-motion
 * powers route transitions, list stagger, number count-up, and micro-interactions; overlays keep
 * Chakra's native (exit-aware) motion. Everything is reduced-motion-safe (F.6) and never fakes
 * data/loading (F.7).
 */

export { PageTransition } from './PageTransition';
export { Stagger, StaggerItem } from './Stagger';
export type { StaggerProps, StaggerItemProps } from './Stagger';
export { Burst } from './Burst';
export type { BurstProps } from './Burst';
export { Pop } from './Pop';
export type { PopProps } from './Pop';
export { DURATION, EASE, STAGGER_STEP, pageVariants, listContainer, listItem } from './tokens';

// Hooks live in @/hooks (repo convention); re-exported here so the motion surface is single-import.
export { useReducedMotion } from '@/hooks/useReducedMotion';
export { useCountUp } from '@/hooks/useCountUp';
export type { UseCountUpOptions } from '@/hooks/useCountUp';
