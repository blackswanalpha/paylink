'use client';

/**
 * SecurityTab — two regions:
 *  - Two-factor authentication: a live Enabled/Not-enabled status (from `user.mfa_enabled`) with the
 *    matching action — set up (MfaEnrollModal) when off, disable (a code-confirm modal) when on.
 *    Enroll/disable optimistically patch the store so the badge flips without a refetch.
 *  - Active sessions: a table of `sessions.list` with an optimistic, confirm-gated revoke.
 */

import { useMemo, useState } from 'react';
import { Badge, Box, HStack, Stack, Text } from '@chakra-ui/react';
import type { LinkMintClient, Session } from '@linkmint/sdk';

import { notify } from '@/lib/notify';
import { reportError } from '@/lib/reportError';
import { useSessions } from '@/hooks/useSessions';
import { useAuthStore } from '@/store/auth';
import { MfaChallengeField } from '@/components/auth/MfaChallengeField';
import { MfaEnrollModal } from '@/components/auth/MfaEnrollModal';
import {
  Button,
  DataTable,
  ErrorBanner,
  Loadable,
  Modal,
  Panel,
  TableSkeleton,
} from '@/components/ui';
import { sessionColumns } from './columns';

/** Confirm dialog for the disable-MFA flow (asks for a current TOTP code). */
function MfaDisableModal({
  client,
  open,
  onClose,
  onDisabled,
}: {
  client: LinkMintClient;
  open: boolean;
  onClose: () => void;
  onDisabled: () => void;
}) {
  const [code, setCode] = useState('');
  const [error, setError] = useState<string | undefined>(undefined);
  const [busy, setBusy] = useState(false);

  function close() {
    setCode('');
    setError(undefined);
    onClose();
  }

  async function submit() {
    setBusy(true);
    setError(undefined);
    try {
      await client.auth.mfaDisable({ code: code.trim() });
      notify.success('Two-factor authentication disabled');
      onDisabled();
      close();
    } catch (err) {
      const { error: e } = reportError(err, { silent: true });
      setError(
        e.code === 'MFA_INVALID'
          ? 'That code is invalid or expired.'
          : e.code === 'MFA_NOT_ENROLLED'
            ? 'Two-factor is not enabled on this account.'
            : e.message,
      );
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal
      open={open}
      onClose={close}
      role="alertdialog"
      size="sm"
      title="Disable two-factor authentication"
      description="Enter a current code from your authenticator to turn off MFA."
      footer={
        <>
          <Button variant="ghost" size="sm" onClick={close}>
            Cancel
          </Button>
          <Button
            variant="solid"
            colorPalette="red"
            size="sm"
            onClick={submit}
            loading={busy}
            disabled={code.trim().length < 6}
          >
            Disable
          </Button>
        </>
      }
    >
      <MfaChallengeField value={code} onChange={setCode} error={error} label="Current code" />
    </Modal>
  );
}

export function SecurityTab({ client }: { client: LinkMintClient }) {
  const { items, loading, error, refresh, revoke } = useSessions(client);
  const mfaEnabled = useAuthStore((s) => s.user?.mfa_enabled ?? false);
  const patchUser = useAuthStore((s) => s.patchUser);
  const [enrollOpen, setEnrollOpen] = useState(false);
  const [disableOpen, setDisableOpen] = useState(false);
  const [revokeTarget, setRevokeTarget] = useState<Session | null>(null);

  const columns = useMemo(() => sessionColumns(setRevokeTarget), []);
  const hasData = items.length > 0;
  const isInitialLoading = loading && !hasData;

  return (
    <Stack gap={8} pt={2}>
      <Panel>
        <Stack gap={4}>
          <Box>
            <HStack gap={2} mb={1}>
              <Text fontFamily="heading" fontWeight="600">
                Two-factor authentication
              </Text>
              <Badge colorPalette={mfaEnabled ? 'emerald' : 'gray'} variant="subtle">
                {mfaEnabled ? 'Enabled' : 'Not enabled'}
              </Badge>
            </HStack>
            <Text fontSize="sm" color="fg.muted">
              Add a TOTP authenticator app for an extra layer of security at sign-in.
            </Text>
          </Box>
          <HStack gap={3} flexWrap="wrap">
            {mfaEnabled ? (
              <Button variant="outline" colorPalette="red" onClick={() => setDisableOpen(true)}>
                Disable…
              </Button>
            ) : (
              <Button colorPalette="emerald" variant="outline" onClick={() => setEnrollOpen(true)}>
                Set up authenticator
              </Button>
            )}
          </HStack>
        </Stack>
      </Panel>

      <Panel p={0} overflow="hidden">
        <HStack justify="space-between" px={6} py={4} borderBottomWidth="1px" borderColor="border">
          <Text fontFamily="heading" fontWeight="600" fontSize="lg">
            Active sessions
          </Text>
          <Button variant="outline" size="sm" onClick={refresh} loading={loading && hasData}>
            Refresh
          </Button>
        </HStack>

        {error ? (
          <Box px={6} pt={4}>
            <ErrorBanner error={error} onRetry={refresh} />
          </Box>
        ) : null}

        <Loadable
          loading={isInitialLoading}
          error={error}
          isEmpty={!hasData}
          hasData={hasData}
          skeleton={<TableSkeleton rows={3} label="Sessions table" />}
          empty={
            <Box p={6}>
              <Text fontSize="sm" color="fg.muted">
                No active sessions.
              </Text>
            </Box>
          }
        >
          <DataTable
            columns={columns}
            rows={items}
            rowKey={(s) => s.session_id}
            caption="Active sessions"
          />
        </Loadable>
      </Panel>

      <MfaEnrollModal
        open={enrollOpen}
        onClose={() => setEnrollOpen(false)}
        onEnrolled={() => patchUser({ mfa_enabled: true })}
      />
      <MfaDisableModal
        client={client}
        open={disableOpen}
        onClose={() => setDisableOpen(false)}
        onDisabled={() => patchUser({ mfa_enabled: false })}
      />

      <Modal
        open={revokeTarget !== null}
        onClose={() => setRevokeTarget(null)}
        role="alertdialog"
        size="sm"
        title={revokeTarget?.current ? 'Sign out this device?' : 'Revoke this session?'}
        description={
          revokeTarget?.current
            ? 'This is the device you are using — revoking it signs you out here.'
            : 'That device will be signed out immediately.'
        }
        footer={
          <>
            <Button variant="ghost" size="sm" onClick={() => setRevokeTarget(null)}>
              Cancel
            </Button>
            <Button
              variant="solid"
              colorPalette="red"
              size="sm"
              onClick={() => {
                if (revokeTarget) {
                  void revoke(revokeTarget.session_id);
                  setRevokeTarget(null);
                }
              }}
            >
              Revoke
            </Button>
          </>
        }
      >
        <Text fontSize="sm" color="fg.muted">
          {revokeTarget?.user_agent ?? 'Unknown device'}
        </Text>
      </Modal>
    </Stack>
  );
}
