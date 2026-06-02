/** Dashboard navigation model. `live: false` items are PLANNED (F.7) — shown but not navigable. */

import type { Icon } from 'react-feather';
import { CreditCard, Grid, Link2, Settings } from 'react-feather';

export interface NavItem {
  label: string;
  href: string;
  icon: Icon;
  /** Whether the route is built and navigable today. */
  live: boolean;
}

export const NAV_ITEMS: NavItem[] = [
  { label: 'Overview', href: '/dashboard', icon: Grid, live: true },
  { label: 'PayLinks', href: '/dashboard/paylinks', icon: Link2, live: true },
  { label: 'Payments', href: '/dashboard/payments', icon: CreditCard, live: false },
  { label: 'Settings', href: '/dashboard/onboarding', icon: Settings, live: false },
];
