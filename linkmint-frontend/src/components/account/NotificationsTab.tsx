'use client';

/**
 * NotificationsTab — per-channel + per-event notification preferences (work10), backed by
 * notification-service. Two groups of switches: Channels (how — in-app/email/SMS) and Events (what —
 * the paylink and payment event kinds). Toggling is optimistic via `useNotificationPreferences`,
 * which persists each change and reconciles on failure.
 *
 * These preferences are scoped by the creator address, so this tab uses the HS256/dashboard `client`
 * (the same one the Topbar bell uses) — not the RS256 identity client the other Account tabs take.
 */

import { Box, Stack, Switch, Text } from '@chakra-ui/react';
import type { LinkMintClient, NotificationChannel, NotificationEventKind } from '@linkmint/sdk';

import { useNotificationPreferences } from '@/hooks/useNotificationPreferences';
import { ErrorBanner, FormSkeleton, Loadable, Panel } from '@/components/ui';

const CHANNELS: { key: NotificationChannel; label: string; help: string }[] = [
  {
    key: 'in_app',
    label: 'In-app',
    help: 'Show in the notification center (the bell in the top bar).',
  },
  { key: 'email', label: 'Email', help: 'Send to your email address.' },
  { key: 'sms', label: 'SMS', help: 'Send a text to your phone.' },
];

const EVENTS: { key: NotificationEventKind; label: string; help: string }[] = [
  { key: 'paylink.created', label: 'PayLink created', help: 'When a new PayLink is created.' },
  {
    key: 'paylink.verified',
    label: 'PayLink settled',
    help: 'When a PayLink is verified on-chain.',
  },
  { key: 'paylink.cancelled', label: 'PayLink cancelled', help: 'When a PayLink is cancelled.' },
  { key: 'payment.failed', label: 'Payment failed', help: 'When a payment attempt fails.' },
];

function ToggleRow({
  label,
  help,
  checked,
  onChange,
}: {
  label: string;
  help: string;
  checked: boolean;
  onChange: (value: boolean) => void;
}) {
  return (
    <Switch.Root
      checked={checked}
      onCheckedChange={(e) => onChange(e.checked)}
      colorPalette="emerald"
      display="flex"
      width="full"
      justifyContent="space-between"
      alignItems="flex-start"
      gap={4}
    >
      <Box>
        <Switch.Label fontSize="sm" fontWeight="500" mb={0.5}>
          {label}
        </Switch.Label>
        <Text fontSize="xs" color="fg.muted">
          {help}
        </Text>
      </Box>
      <Switch.HiddenInput />
      <Switch.Control flexShrink={0}>
        <Switch.Thumb />
      </Switch.Control>
    </Switch.Root>
  );
}

export function NotificationsTab({ client }: { client: LinkMintClient }) {
  const { prefs, loading, error, refresh, setChannel, setEvent } =
    useNotificationPreferences(client);
  const hasData = prefs !== null;

  return (
    <Stack gap={6} pt={2}>
      {error ? <ErrorBanner error={error} onRetry={refresh} /> : null}

      <Loadable
        loading={loading && !hasData}
        error={error}
        isEmpty={false}
        hasData={hasData}
        skeleton={<FormSkeleton fields={3} />}
        empty={null}
      >
        {prefs ? (
          <>
            <Panel>
              <Stack gap={5}>
                <Box>
                  <Text fontFamily="heading" fontWeight="600" fontSize="lg">
                    Channels
                  </Text>
                  <Text fontSize="sm" color="fg.muted">
                    How you want to be notified.
                  </Text>
                </Box>
                {CHANNELS.map((c) => (
                  <ToggleRow
                    key={c.key}
                    label={c.label}
                    help={c.help}
                    checked={prefs.channels[c.key]}
                    onChange={(v) => setChannel(c.key, v)}
                  />
                ))}
              </Stack>
            </Panel>

            <Panel>
              <Stack gap={5}>
                <Box>
                  <Text fontFamily="heading" fontWeight="600" fontSize="lg">
                    Events
                  </Text>
                  <Text fontSize="sm" color="fg.muted">
                    What you want to be notified about.
                  </Text>
                </Box>
                {EVENTS.map((ev) => (
                  <ToggleRow
                    key={ev.key}
                    label={ev.label}
                    help={ev.help}
                    checked={prefs.events[ev.key]}
                    onChange={(v) => setEvent(ev.key, v)}
                  />
                ))}
              </Stack>
            </Panel>
          </>
        ) : null}
      </Loadable>
    </Stack>
  );
}
