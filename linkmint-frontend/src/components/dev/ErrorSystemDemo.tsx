'use client';

/**
 * ErrorSystemDemo — a dev-only harness for the work04 error system. Fire each error class and watch
 * where it surfaces: inline (below), a toast (top-right), or a global modal — and crash a child to
 * prove the React ErrorBoundary catches it. Used by the kitchen-sink gallery to make `/verify` a
 * one-click affair without needing the live backend. Synthetic SDK errors only (dev fixtures).
 */

import { useState } from 'react';
import { Button, HStack, Stack, Text } from '@chakra-ui/react';
import { createApiError, LinkMintConnectionError, type ApiErrorInit } from '@linkmint/sdk';
import type { DisplayError } from '@/lib/errors';
import { reportError } from '@/lib/reportError';
import { useErrorHandler } from '@/hooks/useErrorHandler';
import { useRetry } from '@/hooks/useRetry';
import { ErrorBanner, ErrorBoundary, ErrorFallback, KycGate } from '@/components/ui';

/** Build a synthetic API error (the SDK's typed hierarchy), so it flows through the real pipeline. */
function apiError(
  status: number,
  code: string,
  message: string,
  extra?: Partial<ApiErrorInit>,
): Error {
  return createApiError({
    status,
    code,
    message,
    details: {},
    traceId: `trace-${status}-demo`,
    requestId: `req-${status}-demo`,
    ...extra,
  });
}

/** A child that throws on render — to exercise the boundary. */
function Boom(): never {
  throw new Error('Kitchen-sink: intentional render crash');
}

export function ErrorSystemDemo() {
  const { inlineError, report, clear } = useErrorHandler({ surface: 'inline' });
  const { retry, cooldown, noteError } = useRetry({ run: async () => clear() });
  const [kycInline, setKycInline] = useState<DisplayError | null>(null);
  const [crash, setCrash] = useState(false);

  const fireInline = (err: Error): void => {
    const { error } = report(err);
    noteError(error); // starts the 429 countdown when a Retry-After is present (no-op otherwise)
  };

  return (
    <Stack gap={6}>
      <Text color="fg.muted" fontSize="sm">
        Fire each error class and watch where it surfaces — inline below, a toast (top-right), or a
        global modal. Every API error carries a copyable trace id.
      </Text>

      {/* Inline surfaces */}
      <Stack gap={2}>
        <Text fontWeight="medium" fontSize="sm">
          Inline (renders below)
        </Text>
        <HStack gap={2} wrap="wrap">
          <Button
            size="sm"
            variant="outline"
            onClick={() =>
              fireInline(apiError(400, 'INVALID_PAYLOAD', 'The amount must be a positive integer.'))
            }
          >
            400 bad request
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={() =>
              fireInline(apiError(403, 'FORBIDDEN', 'You don’t have access to this resource.'))
            }
          >
            403 forbidden
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={() =>
              fireInline(apiError(404, 'PAYLINK_NOT_FOUND', 'No PayLink with that id.'))
            }
          >
            404 not found
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={() =>
              fireInline(
                apiError(409, 'PAYLINK_ALREADY_SETTLED', 'This PayLink is already settled.'),
              )
            }
          >
            409 conflict
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={() =>
              fireInline(
                apiError(429, 'RATE_LIMITED', 'Too many requests — slow down.', { retryAfter: 10 }),
              )
            }
          >
            429 rate limit
          </Button>
          <Button size="sm" variant="ghost" onClick={clear}>
            Clear
          </Button>
        </HStack>
        {inlineError ? (
          <ErrorBanner error={inlineError} onRetry={retry} retryCooldown={cooldown} />
        ) : null}
      </Stack>

      {/* Toast surfaces */}
      <Stack gap={2}>
        <Text fontWeight="medium" fontSize="sm">
          Toast (top-right)
        </Text>
        <HStack gap={2} wrap="wrap">
          <Button
            size="sm"
            variant="outline"
            onClick={() =>
              reportError(apiError(500, 'INTERNAL_ERROR', 'Something broke on our end.'))
            }
          >
            5xx server error
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={() => reportError(new LinkMintConnectionError('Failed to fetch'))}
          >
            transport failure
          </Button>
        </HStack>
      </Stack>

      {/* Global modals */}
      <Stack gap={2}>
        <Text fontWeight="medium" fontSize="sm">
          Global modal
        </Text>
        <HStack gap={2} wrap="wrap">
          <Button
            size="sm"
            variant="outline"
            onClick={() => reportError(apiError(401, 'UNAUTHORIZED', 'Your session has expired.'))}
          >
            401 re-auth
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={() =>
              reportError(apiError(402, 'KYC_REQUIRED', 'Verify your identity to continue.'))
            }
          >
            402 KYC modal
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={() => {
              const { error } = reportError(
                apiError(
                  402,
                  'KYC_REQUIRED',
                  'Verify your identity to create PayLinks above your tier limit.',
                ),
                { surface: 'inline' },
              );
              setKycInline(error);
            }}
          >
            402 inline gate
          </Button>
        </HStack>
        {kycInline ? <KycGate error={kycInline} onVerify={() => setKycInline(null)} /> : null}
      </Stack>

      {/* Render-crash boundary */}
      <Stack gap={2}>
        <Text fontWeight="medium" fontSize="sm">
          Render crash (caught by a local ErrorBoundary)
        </Text>
        <ErrorBoundary
          fallback={({ id, reset }) => (
            <ErrorFallback
              id={id}
              title="This section crashed"
              description="The local ErrorBoundary caught a render error. Try again to recover just this section."
              onReset={() => {
                setCrash(false);
                reset();
              }}
              showHome={false}
            />
          )}
        >
          {crash ? <Boom /> : <Text color="fg.muted">Boundary OK — nothing thrown yet.</Text>}
        </ErrorBoundary>
        <HStack>
          <Button size="sm" variant="outline" onClick={() => setCrash(true)}>
            Crash this section
          </Button>
        </HStack>
      </Stack>

      <Text color="fg.muted" fontSize="xs">
        Offline banner: toggle DevTools → Network → “Offline” to see the app-wide connectivity
        banner.
      </Text>
    </Stack>
  );
}
