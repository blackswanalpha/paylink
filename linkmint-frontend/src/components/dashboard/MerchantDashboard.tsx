'use client';

/**
 * MerchantDashboard — the flagship Ivory Premium screen (frontendfeature.md §3.3). Builds the single
 * SDK client from the server-minted token (client-side, so the base URL resolves to the app origin),
 * loads the merchant's PayLinks (LIVE/work01), and renders headline metrics + a recent-activity
 * sparkline + a premium PayLinks table. All data is real; nothing is faked (F.7).
 */

import { useEffect, useMemo, useState } from 'react';
import NextLink from 'next/link';
import { Button, Grid, HStack, SimpleGrid, Stack, Text } from '@chakra-ui/react';
import { Inbox, PlusCircle, RefreshCw } from 'react-feather';
import type { LinkMintClient, PayLink } from '@linkmint/sdk';
import { createLinkMintClient } from '@/lib/linkmint';
import { usePayLinks } from '@/hooks/usePayLinks';
import { AppShell } from '@/components/shell/AppShell';
import {
  AddressChip,
  AmountDisplay,
  DataTable,
  EmptyState,
  ErrorBanner,
  MetricCard,
  MetricCardSkeleton,
  PageHeader,
  Panel,
  PayLinkStatusPill,
  TableRowsSkeleton,
  type DataTableColumn,
} from '@/components/ui';

const RECENT_LIMIT = 8;

function formatShortDate(iso: string): string {
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

const RECENT_COLUMNS: DataTableColumn<PayLink>[] = [
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

export function MerchantDashboard({
  initialToken,
  creatorAddr,
}: {
  initialToken: string;
  creatorAddr: string;
}) {
  const [client, setClient] = useState<LinkMintClient | null>(null);
  useEffect(() => {
    setClient(createLinkMintClient(initialToken));
  }, [initialToken]);

  const { items, aggregates, loading, error, refresh } = usePayLinks(client, creatorAddr);
  const currency = aggregates.currency ?? 'KES';

  const recent = useMemo(
    () =>
      [...items]
        .sort((a, b) => Date.parse(b.created_at) - Date.parse(a.created_at))
        .slice(0, RECENT_LIMIT),
    [items],
  );

  const initializing = !client || (loading && items.length === 0);

  return (
    <AppShell>
      <Stack gap={8}>
        <PageHeader
          title="Overview"
          subtitle="Your PayLinks, settlements, and on-chain activity at a glance."
          actions={
            <>
              <Button
                variant="outline"
                size="sm"
                onClick={refresh}
                loading={loading && items.length > 0}
              >
                <HStack gap={2}>
                  <RefreshCw size={15} />
                  <Text>Refresh</Text>
                </HStack>
              </Button>
              <Button asChild variant="solid" colorPalette="emerald" size="sm">
                <NextLink href="/">
                  <HStack gap={2}>
                    <PlusCircle size={15} />
                    <Text>Create PayLink</Text>
                  </HStack>
                </NextLink>
              </Button>
            </>
          }
        />

        {error ? <ErrorBanner error={error} onRetry={refresh} /> : null}

        {/* Metrics */}
        <SimpleGrid columns={{ base: 1, sm: 2, lg: 4 }} gap={5}>
          {initializing ? (
            <>
              <MetricCardSkeleton />
              <MetricCardSkeleton />
              <MetricCardSkeleton />
              <MetricCardSkeleton />
            </>
          ) : (
            <>
              <MetricCard
                label="Total settled"
                value={
                  <AmountDisplay
                    amountMinor={aggregates.totalSettledMinor}
                    currency={currency}
                    size="md"
                  />
                }
                sparkline={aggregates.sparkline}
              />
              <MetricCard label="Active PayLinks" value={String(aggregates.activeCount)} />
              <MetricCard label="Pending settlement" value={String(aggregates.pendingCount)} />
              <MetricCard label="Total PayLinks" value={String(aggregates.total)} />
            </>
          )}
        </SimpleGrid>

        {/* Recent PayLinks */}
        <Panel p={0} overflow="hidden">
          <HStack
            justify="space-between"
            px={6}
            py={4}
            borderBottomWidth="1px"
            borderColor="border"
          >
            <Text fontFamily="heading" fontWeight="600" fontSize="lg">
              Recent PayLinks
            </Text>
            {!initializing && items.length > 0 ? (
              <Text fontSize="sm" color="fg.muted">
                Showing {recent.length} of {items.length}
              </Text>
            ) : null}
          </HStack>

          {initializing ? (
            <Stack p={6}>
              <TableRowsSkeleton rows={5} />
            </Stack>
          ) : items.length === 0 ? (
            <EmptyState
              icon={<Inbox size={24} />}
              title={error ? 'Could not load PayLinks' : 'No PayLinks yet'}
              description={
                error
                  ? 'Try refreshing once the local stack is up.'
                  : 'Create your first PayLink and share it — it will appear here as it moves to settled.'
              }
              action={
                <Button asChild variant="solid" colorPalette="emerald" size="sm">
                  <NextLink href="/">
                    <HStack gap={2}>
                      <PlusCircle size={15} />
                      <Text>Create PayLink</Text>
                    </HStack>
                  </NextLink>
                </Button>
              }
            />
          ) : (
            <DataTable
              columns={RECENT_COLUMNS}
              rows={recent}
              rowKey={(pl) => pl.pl_id}
              interactive
              caption="Recent PayLinks"
            />
          )}
        </Panel>

        <Grid>
          <Text fontSize="xs" color="fg.muted">
            Settlement amounts and analytics shown here are derived from your live PayLinks. Richer
            reporting (revenue series, conversion, rail mix) arrives with the analytics service —
            tracked as PLANNED in frontendfeature.md §3.3.
          </Text>
        </Grid>
      </Stack>
    </AppShell>
  );
}
