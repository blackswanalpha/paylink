'use client';

/** Render a normalized DisplayError — the standard LinkMint envelope (code / message / trace). */

import { Alert, Code, Stack, Text } from '@chakra-ui/react';
import type { DisplayError } from '@/lib/errors';

export function ErrorBanner({ error }: { error: DisplayError }) {
  const status = error.kind === 'transport' ? 'warning' : 'error';
  return (
    <Alert.Root status={status} borderRadius="md">
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
              <Text fontSize="xs" color="fg.muted">
                trace: <Code>{error.traceId}</Code>
              </Text>
            ) : null}
          </Stack>
        </Alert.Description>
      </Alert.Content>
    </Alert.Root>
  );
}
