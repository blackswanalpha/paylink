'use client';

/**
 * ErrorFallback — the branded, calm full-page fallback shown for a render crash (work04). Shared by
 * the React `ErrorBoundary` and the Next route files (`error.tsx`, segment `error.tsx`). A render
 * crash has no error envelope, so there's no `trace_id`; instead it shows a short, copyable error id
 * (a generated id, or a route error `digest`) for support to correlate by id + timestamp.
 */

import NextLink from 'next/link';
import { Box, Button, HStack, Stack } from '@chakra-ui/react';
import { AlertTriangle, Home, RefreshCw } from 'react-feather';
import { EmptyState } from '@/components/ui/EmptyState';
import { CopyField } from '@/components/ui/CopyField';

export interface ErrorFallbackProps {
  /** Short, copyable id correlating to logs (a render-crash id or a route error digest). */
  id?: string;
  title?: string;
  description?: string;
  /** Invoked by "Try again" (e.g. Next's `reset`). When omitted, the page reloads. */
  onReset?: () => void;
  /** Show the "Go home" link. @default true */
  showHome?: boolean;
}

export function ErrorFallback({
  id,
  title = 'Something went wrong',
  description = 'An unexpected error interrupted this page. You can try again, or head back home.',
  onReset,
  showHome = true,
}: ErrorFallbackProps) {
  const reset = onReset ?? (() => window.location.reload());

  return (
    <Box
      role="alert"
      aria-live="assertive"
      minH="60vh"
      display="grid"
      placeItems="center"
      px={6}
      py={16}
    >
      <Stack gap={4} align="center" maxW="md">
        <EmptyState
          icon={<AlertTriangle size={24} />}
          title={title}
          description={description}
          action={
            <HStack gap={3}>
              <Button colorPalette="emerald" onClick={reset} gap={2}>
                <RefreshCw size={16} /> Try again
              </Button>
              {showHome ? (
                <Button asChild variant="outline" gap={2}>
                  <NextLink href="/">
                    <Home size={16} /> Go home
                  </NextLink>
                </Button>
              ) : null}
            </HStack>
          }
        />
        {id ? <CopyField value={id} label="error id" mono variant="inline" size="sm" /> : null}
      </Stack>
    </Box>
  );
}
