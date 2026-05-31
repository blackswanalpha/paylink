'use client';

/** A colored pill for a PayLink lifecycle status. */

import { Badge } from '@chakra-ui/react';
import type { PayLinkStatus } from '@linkmint/sdk';

const PALETTE: Record<PayLinkStatus, 'gray' | 'yellow' | 'green' | 'red' | 'orange'> = {
  CREATED: 'gray',
  PENDING: 'yellow',
  VERIFIED: 'green',
  FAILED: 'red',
  CANCELLED: 'red',
  EXPIRED: 'orange',
};

export function StatusBadge({ status }: { status: PayLinkStatus }) {
  return (
    <Badge colorPalette={PALETTE[status]} variant="solid">
      {status}
    </Badge>
  );
}
