/**
 * Route template — the App Router re-mounts this on every navigation, so wrapping children in
 * PageTransition replays the page-enter animation on each route change (work05 / ADR-012). A
 * `layout.tsx` would persist instead; `template.tsx` is the idiomatic, SSR-safe spot for enter motion.
 */

import type { ReactNode } from 'react';
import { PageTransition } from '@/motion/PageTransition';

export default function Template({ children }: { children: ReactNode }) {
  return <PageTransition>{children}</PageTransition>;
}
