/**
 * Motion tokens (frontendfeature.md §2.4) for the framer-motion engine — ADR-012.
 *
 * Durations are in SECONDS (framer-motion's unit); the CSS side mirrors them as the Chakra
 * `durations`/`easings` tokens in `theme/system.ts`. `EASE` is the Ivory standard curve
 * `cubic-bezier(.2,.8,.2,1)` expressed as framer's [x1,y1,x2,y2] tuple.
 *
 * Reduced motion is handled GLOBALLY by `<MotionConfig reducedMotion="user">` (Provider.tsx) plus the
 * `prefers-reduced-motion` block in globals.css, so these presets never need a per-use guard (F.6).
 */

import type { Variants } from 'framer-motion';

/** Durations in seconds — fast 120ms · base 200ms · slow 320ms. */
export const DURATION = { fast: 0.12, base: 0.2, slow: 0.32 } as const;

/** The Ivory standard easing curve as a framer cubic-bezier tuple. */
export const EASE: [number, number, number, number] = [0.2, 0.8, 0.2, 1];

/** Seconds between staggered children (list/grid entrance). */
export const STAGGER_STEP = 0.045;

/** Page/route enter: fade + a small rise. Used by `PageTransition` (app/template.tsx). */
export const pageVariants: Variants = {
  hidden: { opacity: 0, y: 8 },
  show: { opacity: 1, y: 0, transition: { duration: DURATION.base, ease: EASE } },
};

/** Stagger container — orchestrates the entrance of its children (`Stagger`). */
export const listContainer: Variants = {
  hidden: {},
  show: { transition: { staggerChildren: STAGGER_STEP } },
};

/** Stagger child — fade + rise (`StaggerItem` / DataTable rows). */
export const listItem: Variants = {
  hidden: { opacity: 0, y: 8 },
  show: { opacity: 1, y: 0, transition: { duration: DURATION.base, ease: EASE } },
};
