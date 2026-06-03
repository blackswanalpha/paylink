'use client';

/**
 * RevealKeyModal — shows a freshly-issued API key's secret EXACTLY once. The secret is never
 * re-fetchable (`full_key` is null on any replay), so the dialog blocks dismissal until the user
 * confirms they've copied it (`disableDismiss`, no close X). On an idempotent replay (`full_key`
 * null) it explains the secret is unavailable and to revoke + re-issue. The secret lives only in this
 * component's props/local state — never persisted.
 */

import { Box, Stack, Text } from '@chakra-ui/react';
import { AlertTriangle } from 'react-feather';
import type { IssueApiKeyResult } from '@linkmint/sdk';

import { Button, CopyField, Modal } from '@/components/ui';

export interface RevealKeyModalProps {
  /** The issued key, or null when the modal is closed. */
  result: IssueApiKeyResult | null;
  onAcknowledge: () => void;
}

export function RevealKeyModal({ result, onAcknowledge }: RevealKeyModalProps) {
  const open = result !== null;
  const hasSecret = Boolean(result?.full_key);

  return (
    <Modal
      open={open}
      onClose={onAcknowledge}
      disableDismiss
      hideCloseButton
      role="alertdialog"
      title="Copy your API key now"
      description="This is the only time the full key is shown."
      footer={
        <Button colorPalette="emerald" onClick={onAcknowledge}>
          I&apos;ve copied it
        </Button>
      }
    >
      <Stack gap={4}>
        {hasSecret && result?.full_key ? (
          <>
            <Box
              role="note"
              display="flex"
              gap={2}
              p={3}
              bg="status.dangerSubtle"
              color="status.danger"
              borderRadius="md"
              fontSize="sm"
            >
              <Box flexShrink={0} mt="2px">
                <AlertTriangle size={16} aria-hidden />
              </Box>
              <Text>
                Store it somewhere safe. For security it&apos;s never shown again — if you lose it,
                revoke this key and issue a new one.
              </Text>
            </Box>
            <CopyField value={result.full_key} label="API key" mono variant="block" />
            <Text fontSize="xs" color="fg.muted">
              Key{' '}
              <Text as="span" fontFamily="mono">
                {result.prefix}…
              </Text>{' '}
              · {result.name}
            </Text>
          </>
        ) : (
          <Text fontSize="sm" color="fg.muted">
            The secret for this key isn&apos;t available (this was a replayed request). Revoke this
            key and issue a new one to get a fresh secret.
          </Text>
        )}
      </Stack>
    </Modal>
  );
}
