'use client';

/**
 * ApiKeysTab — list / issue / revoke scoped API keys. Issuing opens a create modal (org + name +
 * scopes); on success the one-time secret is shown via RevealKeyModal (and the key appears in the
 * list without its secret). Revoke is confirm-gated and optimistic. Org options come from the user's
 * roles (an org is required to scope a key).
 */

import { useMemo, useState, type FormEvent } from 'react';
import { Box, HStack, Input, NativeSelect, Stack, Text } from '@chakra-ui/react';
import { Plus } from 'react-feather';
import type { ApiKey, IssueApiKeyResult, LinkMintClient, Scope } from '@linkmint/sdk';

import { useApiKeys } from '@/hooks/useApiKeys';
import { useAuthStore } from '@/store/auth';
import {
  Button,
  DataTable,
  ErrorBanner,
  FormField,
  Loadable,
  Modal,
  NoApiKeysEmpty,
  Panel,
  TableSkeleton,
} from '@/components/ui';
import { apiKeyColumns } from './columns';
import { RevealKeyModal } from './RevealKeyModal';

const ALL_SCOPES: Scope[] = ['paylinks:read', 'paylinks:write', 'payments:read', 'payments:write'];

export function ApiKeysTab({ client }: { client: LinkMintClient }) {
  const { items, loading, error, refresh, create, revoke } = useApiKeys(client);
  const orgIds = useAuthStore((s) => s.user?.roles.map((r) => r.org_id) ?? []);

  const [createOpen, setCreateOpen] = useState(false);
  const [orgId, setOrgId] = useState(orgIds[0] ?? '');
  const [name, setName] = useState('');
  const [scopes, setScopes] = useState<Set<Scope>>(new Set());
  const [creating, setCreating] = useState(false);
  const [revealResult, setRevealResult] = useState<IssueApiKeyResult | null>(null);
  const [revokeTarget, setRevokeTarget] = useState<ApiKey | null>(null);

  const columns = useMemo(() => apiKeyColumns(setRevokeTarget), []);
  const hasData = items.length > 0;
  const isInitialLoading = loading && !hasData;
  const canCreate = orgIds.length > 0;

  function openCreate() {
    setName('');
    setScopes(new Set());
    setOrgId(orgIds[0] ?? '');
    setCreateOpen(true);
  }

  function toggleScope(scope: Scope) {
    setScopes((prev) => {
      const next = new Set(prev);
      if (next.has(scope)) {
        next.delete(scope);
      } else {
        next.add(scope);
      }
      return next;
    });
  }

  async function submitCreate(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!orgId || name.trim().length === 0) {
      return;
    }
    setCreating(true);
    const result = await create({ org_id: orgId, name: name.trim(), scopes: [...scopes] });
    setCreating(false);
    if (result) {
      setCreateOpen(false);
      setRevealResult(result); // show the one-time secret
    }
  }

  const createCta = (
    <Button colorPalette="emerald" size="sm" gap={2} onClick={openCreate} disabled={!canCreate}>
      <Plus size={15} /> Create key
    </Button>
  );

  return (
    <Stack gap={6} pt={2}>
      <Panel p={0} overflow="hidden">
        <HStack justify="space-between" px={6} py={4} borderBottomWidth="1px" borderColor="border">
          <Box>
            <Text fontFamily="heading" fontWeight="600" fontSize="lg">
              API keys
            </Text>
            <Text fontSize="sm" color="fg.muted">
              Scoped keys for programmatic access via the SDK.
            </Text>
          </Box>
          {createCta}
        </HStack>

        {!canCreate ? (
          <Box px={6} pt={4}>
            <Text fontSize="sm" color="fg.muted">
              Create an organization first (Organizations tab) — a key is scoped to an org.
            </Text>
          </Box>
        ) : null}

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
          skeleton={<TableSkeleton rows={4} label="API keys table" />}
          empty={<NoApiKeysEmpty action={canCreate ? createCta : undefined} />}
        >
          <DataTable
            columns={columns}
            rows={items}
            rowKey={(k) => k.api_key_id}
            caption="API keys"
          />
        </Loadable>
      </Panel>

      {/* Create key modal */}
      <Modal
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        title="Create API key"
        description="Scope the key to an organization and the actions it may perform."
        footer={
          <>
            <Button variant="ghost" size="sm" onClick={() => setCreateOpen(false)}>
              Cancel
            </Button>
            <Button
              type="submit"
              form="create-api-key-form"
              colorPalette="emerald"
              size="sm"
              loading={creating}
              loadingText="Creating…"
              disabled={!orgId || name.trim().length === 0}
            >
              Create key
            </Button>
          </>
        }
      >
        <form id="create-api-key-form" onSubmit={submitCreate} noValidate>
          <Stack gap={4}>
            <FormField label="Organization" required>
              <NativeSelect.Root>
                <NativeSelect.Field value={orgId} onChange={(e) => setOrgId(e.target.value)}>
                  {orgIds.map((id) => (
                    <option key={id} value={id}>
                      {id}
                    </option>
                  ))}
                </NativeSelect.Field>
                <NativeSelect.Indicator />
              </NativeSelect.Root>
            </FormField>

            <FormField label="Name" required helperText="A label to recognize this key later.">
              <Input
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g. CI pipeline"
                maxLength={120}
              />
            </FormField>

            <FormField label="Scopes" helperText="What this key is allowed to do.">
              <HStack flexWrap="wrap" gap={2}>
                {ALL_SCOPES.map((scope) => {
                  const on = scopes.has(scope);
                  return (
                    <Button
                      key={scope}
                      type="button"
                      size="xs"
                      variant={on ? 'solid' : 'outline'}
                      colorPalette="emerald"
                      onClick={() => toggleScope(scope)}
                      aria-pressed={on}
                    >
                      {scope}
                    </Button>
                  );
                })}
              </HStack>
            </FormField>
          </Stack>
        </form>
      </Modal>

      {/* One-time secret reveal */}
      <RevealKeyModal result={revealResult} onAcknowledge={() => setRevealResult(null)} />

      {/* Revoke confirm */}
      <Modal
        open={revokeTarget !== null}
        onClose={() => setRevokeTarget(null)}
        role="alertdialog"
        size="sm"
        title="Revoke this API key?"
        description="Any integration using it will stop working immediately. This can't be undone."
        footer={
          <>
            <Button variant="ghost" size="sm" onClick={() => setRevokeTarget(null)}>
              Keep it
            </Button>
            <Button
              variant="solid"
              colorPalette="red"
              size="sm"
              onClick={() => {
                if (revokeTarget) {
                  void revoke(revokeTarget.api_key_id);
                  setRevokeTarget(null);
                }
              }}
            >
              Revoke key
            </Button>
          </>
        }
      >
        {revokeTarget ? (
          <Text fontSize="sm" color="fg.muted">
            {revokeTarget.name} ·{' '}
            <Text as="span" fontFamily="mono">
              {revokeTarget.prefix}…
            </Text>
          </Text>
        ) : null}
      </Modal>
    </Stack>
  );
}
