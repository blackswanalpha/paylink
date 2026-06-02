'use client';

/**
 * KycGate — the inline (contextual) form of the 402 `KYC_REQUIRED` gate, for screens that opt into
 * `reportError(err, { surface: 'inline' })` instead of the app-wide modal (work04, "both configurable").
 * It's the natural fit for the live create-flow 402 (a tier-0 merchant over threshold), rendered right
 * where the action happened.
 *
 * Phase-honesty (F.7): the "Verify identity" CTA is a SEAM to work15 — it doesn't fake verification.
 */

import { Button, Stack } from '@chakra-ui/react';
import { Shield } from 'react-feather';
import type { DisplayError } from '@/lib/errors';
import { Panel } from '@/components/ui/Panel';
import { EmptyState } from '@/components/ui/EmptyState';
import { CopyField } from '@/components/ui/CopyField';

export interface KycGateProps {
  error: DisplayError;
  /** Called when the user clicks "Verify identity" — SEAM to work15. */
  onVerify?: () => void;
}

export function KycGate({ error, onVerify }: KycGateProps) {
  return (
    <Panel
      role="alert"
      aria-live="assertive"
      p={4}
      bg="status.pendingSubtle"
      borderColor="status.pending"
    >
      <Stack align="center" gap={3}>
        <EmptyState
          icon={<Shield size={22} />}
          title="Verification required"
          description={
            error.message || 'Identity verification is required before you can continue.'
          }
          action={
            <Button
              colorPalette="emerald"
              onClick={() => {
                // SEAM(work15): open the KYC flow once it exists.
                onVerify?.();
              }}
            >
              Verify identity
            </Button>
          }
        />
        {error.traceId ? (
          <CopyField value={error.traceId} label="trace id" mono variant="inline" size="sm" />
        ) : null}
      </Stack>
    </Panel>
  );
}
