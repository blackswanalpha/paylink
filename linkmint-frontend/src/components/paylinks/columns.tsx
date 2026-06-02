'use client';

/**
 * Shared PayLink table columns + the optimistic-cancel row action (work06). Used by the dashboard's
 * "Recent PayLinks" table and the /dashboard/paylinks management page so both share one definition.
 *
 * Columns are data (the `DataTable` contract). The actions column renders a keyboard-reachable ⋯ Menu
 * with a single destructive "Cancel PayLink" action, shown only for cancellable (CREATED/PENDING)
 * PayLinks. It delegates to `onRequestCancel` so the page owns the confirm dialog + the actual
 * mutation (`usePayLinks().cancel`) — keeping this module presentational.
 */

import { Text } from '@chakra-ui/react';
import { MoreHorizontal, XCircle } from 'react-feather';
import type { PayLink, PayLinkStatus } from '@linkmint/sdk';
import {
  AddressChip,
  AmountDisplay,
  IconButton,
  Menu,
  PayLinkStatusPill,
  type DataTableColumn,
} from '@/components/ui';

/** Short, locale-aware date for table cells. */
export function formatShortDate(iso: string): string {
  const t = Date.parse(iso);
  if (Number.isNaN(t)) {
    return '—';
  }
  return new Date(t).toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

/** A PayLink can be cancelled only before it reaches a terminal state. */
export function isCancellable(status: PayLinkStatus): boolean {
  return status === 'CREATED' || status === 'PENDING';
}

/** The canonical PayLink data columns (no actions). */
export const PAYLINK_COLUMNS: DataTableColumn<PayLink>[] = [
  {
    key: 'pl_id',
    header: 'PayLink',
    render: (pl) => <AddressChip value={pl.pl_id} label="PayLink id" />,
  },
  {
    key: 'receiver',
    header: 'Receiver',
    render: (pl) => <AddressChip value={pl.receiver} label="receiver" />,
  },
  {
    key: 'amount',
    header: 'Amount',
    align: 'end',
    sortable: true,
    sortValue: (pl) => pl.amount,
    render: (pl) => <AmountDisplay amountMinor={pl.amount} currency={pl.currency} size="sm" />,
  },
  {
    key: 'status',
    header: 'Status',
    render: (pl) => <PayLinkStatusPill status={pl.status} />,
  },
  {
    key: 'created',
    header: 'Created',
    sortable: true,
    sortValue: (pl) => Date.parse(pl.created_at),
    render: (pl) => (
      <Text fontSize="sm" color="fg.muted" whiteSpace="nowrap">
        {formatShortDate(pl.created_at)}
      </Text>
    ),
  },
];

/** The trailing actions column: a ⋯ Menu with Cancel, shown only for cancellable PayLinks. */
export function cancelActionColumn(
  onRequestCancel: (pl: PayLink) => void,
): DataTableColumn<PayLink> {
  return {
    key: 'actions',
    header: '',
    align: 'end',
    width: '48px',
    render: (pl) =>
      isCancellable(pl.status) ? (
        <Menu
          trigger={
            <IconButton aria-label={`Actions for PayLink ${pl.pl_id}`} variant="ghost" size="sm">
              <MoreHorizontal size={16} />
            </IconButton>
          }
          items={[
            {
              value: 'cancel',
              label: 'Cancel PayLink',
              icon: <XCircle size={14} />,
              tone: 'danger',
            },
          ]}
          onSelect={(value) => {
            if (value === 'cancel') {
              onRequestCancel(pl);
            }
          }}
        />
      ) : null,
  };
}

/** Data columns + the cancel action column (the common table shape for PayLink surfaces). */
export function payLinkColumnsWithCancel(
  onRequestCancel: (pl: PayLink) => void,
): DataTableColumn<PayLink>[] {
  return [...PAYLINK_COLUMNS, cancelActionColumn(onRequestCancel)];
}
