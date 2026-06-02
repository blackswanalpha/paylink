'use client';

/**
 * StatusPill — the one component that renders any backend lifecycle status, reading the
 * `status.*` semantic tokens from the Ivory Premium theme (frontendfeature.md §2.6). Color is never
 * the only signal: every pill pairs a tinted background with a solid dot and a text label (F.6).
 */

import { Box, HStack, Text } from '@chakra-ui/react';
import type { PayLinkStatus, PaymentStatus } from '@linkmint/sdk';

export type StatusKind = 'success' | 'pending' | 'neutral' | 'danger' | 'expired';

const TONE: Record<StatusKind, { fg: string; bg: string }> = {
  success: { fg: 'status.success', bg: 'status.successSubtle' },
  pending: { fg: 'status.pending', bg: 'status.pendingSubtle' },
  neutral: { fg: 'status.neutral', bg: 'status.neutralSubtle' },
  danger: { fg: 'status.danger', bg: 'status.dangerSubtle' },
  expired: { fg: 'status.expired', bg: 'status.expiredSubtle' },
};

const PAYLINK_KIND: Record<PayLinkStatus, StatusKind> = {
  CREATED: 'neutral',
  PENDING: 'pending',
  VERIFIED: 'success',
  FAILED: 'danger',
  CANCELLED: 'danger',
  EXPIRED: 'expired',
};

const PAYMENT_KIND: Record<PaymentStatus, StatusKind> = {
  AWAITING_PAYMENT: 'pending',
  SETTLED: 'success',
  FAILED: 'danger',
  CANCELLED: 'danger',
};

export function statusKindForPayLink(status: PayLinkStatus): StatusKind {
  return PAYLINK_KIND[status];
}

export function statusKindForPayment(status: PaymentStatus): StatusKind {
  return PAYMENT_KIND[status];
}

export interface StatusPillProps {
  kind: StatusKind;
  label: string;
  /** Hide the leading dot (e.g. in dense tables). */
  hideDot?: boolean;
}

export function StatusPill({ kind, label, hideDot }: StatusPillProps) {
  const tone = TONE[kind];
  return (
    <HStack
      as="span"
      display="inline-flex"
      gap={2}
      px={3}
      py={1}
      bg={tone.bg}
      color={tone.fg}
      borderRadius="full"
      fontSize="xs"
      fontWeight="600"
      letterSpacing="0.02em"
      lineHeight="1"
      whiteSpace="nowrap"
    >
      {hideDot ? null : <Box as="span" w="6px" h="6px" borderRadius="full" bg={tone.fg} />}
      <Text as="span">{label}</Text>
    </HStack>
  );
}

/** Convenience pill for a PayLink lifecycle status. */
export function PayLinkStatusPill({ status }: { status: PayLinkStatus }) {
  return <StatusPill kind={PAYLINK_KIND[status]} label={status} />;
}

/** Convenience pill for a payment lifecycle status. */
export function PaymentStatusPill({ status }: { status: PaymentStatus }) {
  return <StatusPill kind={PAYMENT_KIND[status]} label={status.replace(/_/g, ' ')} />;
}
