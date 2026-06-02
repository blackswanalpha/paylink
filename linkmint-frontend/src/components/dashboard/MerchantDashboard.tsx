'use client';

/**
 * MerchantDashboard — the flagship Ivory Premium screen (frontendfeature.md §3.3). Builds the single
 * SDK client from the server-minted token (client-side, so the base URL resolves to the app origin),
 * loads the merchant's PayLinks (LIVE/work01), and renders headline metrics + a recent-activity
 * sparkline + a premium PayLinks table.
 *
 * Loading / empty / optimistic states run through the work06 system: `Loadable` sequences
 * skeleton → empty → data (and keeps data on refresh, no skeleton flash), the empty state comes from
 * the branded catalog, and a row-level Cancel is optimistic (flip → reconcile → rollback on error,
 * confirmed via an alert dialog). All data is real; nothing is faked (F.7).
 */

import { useEffect, useMemo, useState } from 'react';
import NextLink from 'next/link';
import { Button, Grid, HStack, SimpleGrid, Stack, Text } from '@chakra-ui/react';
import { PlusCircle, RefreshCw } from 'react-feather';
import type { LinkMintClient, PayLink } from '@linkmint/sdk';
import { createLinkMintClient } from '@/lib/linkmint';
import { usePayLinks } from '@/hooks/usePayLinks';
import { useAppStore } from '@/store/app';
import { AppShell } from '@/components/shell/AppShell';
import { payLinkColumnsWithCancel } from '@/components/paylinks/columns';
import { CancelPayLinkModal } from '@/components/paylinks/CancelPayLinkModal';
import { Stagger, StaggerItem } from '@/motion';
import {
  AmountDisplay,
  DataTable,
  ErrorBanner,
  Loadable,
  MetricCard,
  MetricGridSkeleton,
  NoPayLinksEmpty,
  PageHeader,
  Panel,
  TableSkeleton,
} from '@/components/ui';

const RECENT_LIMIT = 8;

export function MerchantDashboard({
  initialToken,
  creatorAddr,
}: {
  initialToken: string;
  creatorAddr: string;
}) {
  const [client, setClient] = useState<LinkMintClient | null>(null);
  useEffect(() => {
    const c = createLinkMintClient(initialToken);
    setClient(c);
    // Also expose globally so the shell's notification bell (Topbar) can drive the inbox (work07).
    useAppStore.getState().setClient(c);
  }, [initialToken]);

  const { items, aggregates, loading, error, refresh, cancel } = usePayLinks(client, creatorAddr);
  const currency = aggregates.currency ?? 'KES';

  const recent = useMemo(
    () =>
      [...items]
        .sort((a, b) => Date.parse(b.created_at) - Date.parse(a.created_at))
        .slice(0, RECENT_LIMIT),
    [items],
  );

  // The cancel row action requests a confirm; the alert dialog owns the (destructive) mutation.
  const [confirmTarget, setConfirmTarget] = useState<PayLink | null>(null);
  const columns = useMemo(() => payLinkColumnsWithCancel(setConfirmTarget), []);

  const hasData = items.length > 0;
  const isInitialLoading = !client || (loading && !hasData);

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

        {/* Metrics — skeleton on first load, real numbers (count-up + stagger) after. Zeros are real
            data, never an "empty". On a failed initial load Loadable yields to the banner above (F.5). */}
        <Loadable
          loading={isInitialLoading}
          error={error}
          isEmpty={false}
          hasData={hasData}
          skeleton={<MetricGridSkeleton count={4} />}
          empty={null}
        >
          <Stagger>
            <SimpleGrid columns={{ base: 1, sm: 2, lg: 4 }} gap={5}>
              <StaggerItem>
                <MetricCard
                  label="Total settled"
                  countUp={{
                    to: aggregates.totalSettledMinor,
                    format: (n) => <AmountDisplay amountMinor={n} currency={currency} size="md" />,
                  }}
                  sparkline={aggregates.sparkline}
                />
              </StaggerItem>
              <StaggerItem>
                <MetricCard label="Active PayLinks" countUp={{ to: aggregates.activeCount }} />
              </StaggerItem>
              <StaggerItem>
                <MetricCard label="Pending settlement" countUp={{ to: aggregates.pendingCount }} />
              </StaggerItem>
              <StaggerItem>
                <MetricCard label="Total PayLinks" countUp={{ to: aggregates.total }} />
              </StaggerItem>
            </SimpleGrid>
          </Stagger>
        </Loadable>

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
            {hasData ? (
              <Text fontSize="sm" color="fg.muted">
                Showing {recent.length} of {items.length}
              </Text>
            ) : null}
          </HStack>

          <Loadable
            loading={isInitialLoading}
            error={error}
            isEmpty={!hasData}
            hasData={hasData}
            skeleton={<TableSkeleton rows={5} label="PayLinks table" />}
            empty={
              <NoPayLinksEmpty
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
            }
          >
            <DataTable
              columns={columns}
              rows={recent}
              rowKey={(pl) => pl.pl_id}
              interactive
              staggerIn
              caption="Recent PayLinks"
            />
          </Loadable>
        </Panel>

        <Grid>
          <Text fontSize="xs" color="fg.muted">
            Settlement amounts and analytics shown here are derived from your live PayLinks. Richer
            reporting (revenue series, conversion, rail mix) arrives with the analytics service —
            tracked as PLANNED in frontendfeature.md §3.3.
          </Text>
        </Grid>
      </Stack>

      <CancelPayLinkModal
        target={confirmTarget}
        onClose={() => setConfirmTarget(null)}
        onConfirm={(pl) => {
          void cancel(pl.pl_id);
          setConfirmTarget(null);
        }}
      />
    </AppShell>
  );
}
