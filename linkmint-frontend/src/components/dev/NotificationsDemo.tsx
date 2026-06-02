'use client';

/**
 * NotificationsDemo — a dev-gallery panel that drives the governed `notify.*` toast taxonomy (work07):
 * the four kinds plus a promise toast (loading→success/error). Mirrors ErrorSystemDemo. Dev-only.
 */

import { HStack, Stack, Text } from '@chakra-ui/react';
import { Button } from '@/components/ui';
import { notify } from '@/lib/notify';

export function NotificationsDemo() {
  const runPromise = (ok: boolean): void => {
    notify.promise(
      new Promise<{ id: string }>((resolve, reject) =>
        setTimeout(() => (ok ? resolve({ id: 'demo' }) : reject(new Error('failed'))), 1200),
      ),
      {
        loading: 'Working…',
        success: (data) => `Done — ${data.id}`,
        error: 'That action failed',
      },
    );
  };

  return (
    <Stack gap={3}>
      <Text fontSize="sm" color="fg.muted">
        The governed <code>notify.*</code> taxonomy. Toasts are announced via <code>aria-live</code>
        , dismissible, and reduced-motion-aware.
      </Text>
      <HStack gap={2} wrap="wrap">
        <Button
          size="sm"
          variant="outline"
          onClick={() => notify.success('Saved', { description: 'Your changes were saved.' })}
        >
          success
        </Button>
        <Button
          size="sm"
          variant="outline"
          onClick={() => notify.info('Heads up', { description: 'Something worth noting.' })}
        >
          info
        </Button>
        <Button
          size="sm"
          variant="outline"
          onClick={() => notify.warning('Careful', { description: 'This may need attention.' })}
        >
          warning
        </Button>
        <Button
          size="sm"
          variant="outline"
          onClick={() => notify.error('Something broke', { description: 'An action failed.' })}
        >
          error
        </Button>
        <Button size="sm" variant="outline" onClick={() => runPromise(true)}>
          promise → resolve
        </Button>
        <Button size="sm" variant="outline" onClick={() => runPromise(false)}>
          promise → reject
        </Button>
      </HStack>
    </Stack>
  );
}
