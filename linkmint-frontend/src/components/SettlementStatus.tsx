'use client';

/** Step 3 — live settlement view: polls the PayLink and shows the on-chain progress + outcome. */

import { Button, Card, Heading, HStack, Spinner, Stack, Text } from '@chakra-ui/react';
import { CheckCircle, RotateCcw, XCircle } from 'react-feather';
import { useAppStore } from '@/store/app';
import { useSettlementStatus } from '@/hooks/useSettlementStatus';
import { StatusBadge } from './StatusBadge';
import { ErrorBanner } from './ErrorBanner';
import { KeyValueRow } from './KeyValueRow';

export function SettlementStatus() {
  const data = useAppStore((s) => s.data);
  const reset = useAppStore((s) => s.reset);
  const settlement = useSettlementStatus(data?.plId ?? '');

  if (!data) return null;
  const pl = settlement.paylink;
  const status = settlement.status ?? data.initialStatus;

  return (
    <Card.Root>
      <Card.Header>
        <HStack justify="space-between">
          <Heading size="md">Settlement</Heading>
          <StatusBadge status={status} />
        </HStack>
        <Text color="fg.muted" fontSize="sm" mt={1}>
          Watching the PayLink settle on-chain (polling).
        </Text>
      </Card.Header>
      <Card.Body>
        <Stack gap={4}>
          <Stack gap={2}>
            <KeyValueRow label="PayLink" value={data.plId} mono />
            <KeyValueRow label="Votes" value={String(pl?.vote_count ?? 0)} />
            <KeyValueRow
              label="Chain tx"
              value={pl?.chain_tx_hash ?? '—'}
              mono={Boolean(pl?.chain_tx_hash)}
            />
            {pl?.verified_at ? <KeyValueRow label="Verified at" value={pl.verified_at} /> : null}
          </Stack>

          {settlement.isTerminal ? (
            status === 'VERIFIED' ? (
              <HStack color="green.500" gap={2}>
                <CheckCircle size={18} />
                <Text fontWeight="medium">Settled on-chain.</Text>
              </HStack>
            ) : (
              <HStack color="red.500" gap={2}>
                <XCircle size={18} />
                <Text fontWeight="medium">PayLink {status.toLowerCase()}.</Text>
              </HStack>
            )
          ) : (
            <HStack color="fg.muted" gap={2}>
              <Spinner size="sm" />
              <Text>Waiting for payment + on-chain settlement…</Text>
            </HStack>
          )}

          {settlement.error ? <ErrorBanner error={settlement.error} /> : null}
        </Stack>
      </Card.Body>
      <Card.Footer>
        <Button variant="outline" onClick={reset} gap={2}>
          <RotateCcw size={16} /> Start over
        </Button>
      </Card.Footer>
    </Card.Root>
  );
}
