'use client';

/**
 * A colored pill for a PayLink lifecycle status.
 *
 * Now a thin wrapper over the token-driven {@link PayLinkStatusPill} (Ivory Premium, §2.6) so the
 * wizard and the dashboard share one status visual. Kept as `StatusBadge` for the existing imports.
 */

import { PayLinkStatusPill } from '@/components/ui/StatusPill';
import type { PayLinkStatus } from '@linkmint/sdk';

export function StatusBadge({ status }: { status: PayLinkStatus }) {
  return <PayLinkStatusPill status={status} />;
}
