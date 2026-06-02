'use client';

/**
 * useReducedMotion — re-exports framer-motion's SSR-safe `prefers-reduced-motion` hook behind a
 * stable internal path, so the app imports it from `@/hooks/useReducedMotion` (and the impl can be
 * swapped without touching call sites). Returns `true` when the OS requests reduced motion; `false`
 * (motion-on) on the server / first paint.
 *
 * Most motion is gated globally by `<MotionConfig reducedMotion="user">` (Provider.tsx); reach for
 * this hook only for JS-driven motion MotionConfig can't see — e.g. the count-up in useCountUp (F.6).
 */

export { useReducedMotion } from 'framer-motion';
