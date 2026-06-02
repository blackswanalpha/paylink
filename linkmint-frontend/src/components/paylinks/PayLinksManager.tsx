'use client';

/**
 * PayLinksManager — the /dashboard/paylinks management surface (frontendfeature.md §3.3). Lists ALL of
 * the merchant's PayLinks (LIVE/work01) in a sortable table and exercises the full work06 system:
 * a skeleton on first load, the branded empty state for a fresh creator, and an optimistic Cancel
 * (flip → reconcile → rollback on error) confirmed via an alert dialog. SDK-only (F.1); nothing
 * faked (F.7). It reuses the dashboard's data hook, columns, and confirm dialog.
 */

import { useEffect, useMemo, useState } from 'react';
import NextLink from 'next/link';
import { Button, HStack, Stack, Text } from '@chakra-ui/react';
import { PlusCircle, RefreshCw } from 'react-feather';
import type { LinkMintClient, PayLink } from '@linkmint/sdk';
import { createLinkMintClient } from '@/lib/linkmint';
import { usePayLinks } from '@/hooks/usePayLinks';
import { useAppStore } from '@/store/app';
import { AppShell } from '@/components/shell/AppShell';
import { payLinkColumnsWithCancel } from '@/components/paylinks/columns';
import { CancelPayLinkModal } from '@/components/paylinks/CancelPayLinkModal';
import {
  DataTable,
  ErrorBanner,
  Loadable,
  NoPayLinksEmpty,
  PageHeader,
  Panel,
  TableSkeleton,
} from '@/components/ui';

export function PayLinksManager({
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

  const { items, loading, error, refresh, cancel } = usePayLinks(client, creatorAddr);

  // Default display order: newest first. The table's sortable columns let the user re-sort in memory.
  const rows = useMemo(
    () => [...items].sort((a, b) => Date.parse(b.created_at) - Date.parse(a.created_at)),
    [items],
  );

  const [confirmTarget, setConfirmTarget] = useState<PayLink | null>(null);
  const columns = useMemo(() => payLinkColumnsWithCancel(setConfirmTarget), []);

  const hasData = items.length > 0;
  const isInitialLoading = !client || (loading && !hasData);

  const createCta = (
    <Button asChild variant="solid" colorPalette="emerald" size="sm">
      <NextLink href="/">
        <HStack gap={2}>
          <PlusCircle size={15} />
          <Text>Create PayLink</Text>
        </HStack>
      </NextLink>
    </Button>
  );

  return (
    <AppShell>
      <Stack gap={8}>
        <PageHeader
          title="PayLinks"
          subtitle="Every PayLink you've created — sort, inspect, and cancel."
          actions={
            <>
              <Button variant="outline" size="sm" onClick={refresh} loading={loading && hasData}>
                <HStack gap={2}>
                  <RefreshCw size={15} />
                  <Text>Refresh</Text>
                </HStack>
              </Button>
              {createCta}
            </>
          }
        />

        {error ? <ErrorBanner error={error} onRetry={refresh} /> : null}

        <Panel p={0} overflow="hidden">
          <HStack
            justify="space-between"
            px={6}
            py={4}
            borderBottomWidth="1px"
            borderColor="border"
          >
            <Text fontFamily="heading" fontWeight="600" fontSize="lg">
              All PayLinks
            </Text>
            {hasData ? (
              <Text fontSize="sm" color="fg.muted">
                {items.length} total
              </Text>
            ) : null}
          </HStack>

          <Loadable
            loading={isInitialLoading}
            error={error}
            isEmpty={!hasData}
            hasData={hasData}
            skeleton={<TableSkeleton rows={8} label="PayLinks table" />}
            empty={<NoPayLinksEmpty action={createCta} />}
          >
            <DataTable
              columns={columns}
              rows={rows}
              rowKey={(pl) => pl.pl_id}
              interactive
              staggerIn
              caption="All PayLinks"
            />
          </Loadable>
        </Panel>
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
