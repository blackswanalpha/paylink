'use client';

/**
 * ErrorBanner — render a normalized `DisplayError` inline (the standard LinkMint envelope: code /
 * message / trace). work04 extends it into the system's inline surface: a copyable `trace_id`/
 * `request_id` (reusing CopyField's idiom), an optional idempotent-read "Try again" button (with a
 * 429 cooldown), an optional CTA slot, and an explicit `role`+`aria-live` so failures are announced
 * to assistive tech (F.6).
 *
 * Severity is conveyed by the Alert icon + title text, never by color alone.
 */

import type { ReactNode } from 'react';
import { Alert, Button, Code, HStack, Stack, Text } from '@chakra-ui/react';
import type { DisplayError } from '@/lib/errors';
// Deep import (not the @/components/ui barrel) — the barrel re-exports ErrorBanner, so importing it
// here would create a cycle.
import { CopyField } from '@/components/ui/CopyField';

type Severity = 'error' | 'warning' | 'info';

export interface ErrorBannerProps {
  error: DisplayError;
  /** Idempotent-read retry. Supplying this renders a "Try again" button. Omit for mutations. */
  onRetry?: () => void;
  /** Override the auto-detected severity (transport → warning, otherwise error). */
  status?: Severity;
  /** aria-live politeness. Defaults to 'assertive' for errors, 'polite' otherwise. */
  live?: 'assertive' | 'polite';
  /** Optional CTA(s) rendered after the retry button (e.g. a "Go home" link for 403/404). */
  action?: ReactNode;
  /** Seconds left before retry is enabled (429 countdown); disables the retry button while > 0. */
  retryCooldown?: number;
}

export function ErrorBanner({
  error,
  onRetry,
  status,
  live,
  action,
  retryCooldown,
}: ErrorBannerProps) {
  const severity: Severity = status ?? (error.kind === 'transport' ? 'warning' : 'error');
  const role = severity === 'error' ? 'alert' : 'status';
  const liveValue = live ?? (severity === 'error' ? 'assertive' : 'polite');
  const onCooldown = typeof retryCooldown === 'number' && retryCooldown > 0;

  return (
    <Alert.Root status={severity} borderRadius="md" role={role} aria-live={liveValue}>
      <Alert.Indicator />
      <Alert.Content>
        <Alert.Title>{error.title}</Alert.Title>
        <Alert.Description>
          <Stack gap={1} mt={1}>
            <Text>{error.message}</Text>
            {error.code ? (
              <Text fontSize="sm">
                code: <Code>{error.code}</Code>
                {typeof error.status === 'number' ? ` · HTTP ${error.status}` : ''}
              </Text>
            ) : null}
            {error.traceId ? (
              <HStack gap={1.5} fontSize="xs" color="fg.muted">
                <Text as="span">trace</Text>
                <CopyField value={error.traceId} label="trace id" mono variant="inline" size="sm" />
              </HStack>
            ) : null}
            {error.requestId ? (
              <HStack gap={1.5} fontSize="xs" color="fg.muted">
                <Text as="span">request</Text>
                <CopyField
                  value={error.requestId}
                  label="request id"
                  mono
                  variant="inline"
                  size="sm"
                />
              </HStack>
            ) : null}
            {onRetry || action ? (
              <HStack gap={3} mt={2}>
                {onRetry ? (
                  <Button variant="outline" size="sm" onClick={onRetry} disabled={onCooldown}>
                    {onCooldown ? `Try again in ${retryCooldown}s` : 'Try again'}
                  </Button>
                ) : null}
                {action}
              </HStack>
            ) : null}
          </Stack>
        </Alert.Description>
      </Alert.Content>
    </Alert.Root>
  );
}
