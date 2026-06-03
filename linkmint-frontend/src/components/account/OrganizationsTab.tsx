'use client';

/**
 * OrganizationsTab — the user's orgs (from their roles, plus any created here) with per-org member
 * management. Create an org, then "Manage members" loads the member table where you can add (by email
 * or user id + role) or remove (optimistic, confirm-gated; `CANNOT_REMOVE_LAST_OWNER` shows inline).
 */

import { useMemo, useState, type FormEvent } from 'react';
import { Badge, Box, HStack, Input, NativeSelect, Stack, Text } from '@chakra-ui/react';
import { Plus, Users } from 'react-feather';
import type { LinkMintClient, Member, OrgType, Role } from '@linkmint/sdk';

import { useOrganizations, type OrgEntry } from '@/hooks/useOrganizations';
import { useAuthStore } from '@/store/auth';
import {
  Button,
  DataTable,
  ErrorBanner,
  FormField,
  Loadable,
  Modal,
  Panel,
  TableSkeleton,
} from '@/components/ui';
import { memberColumns } from './columns';

const ORG_TYPES: OrgType[] = ['merchant', 'developer', 'admin'];
const ROLES: Role[] = ['owner', 'admin', 'developer', 'operator', 'viewer'];

function orgLabel(org: OrgEntry): string {
  return org.name ?? `Organization ${org.org_id.slice(0, 8)}`;
}

export function OrganizationsTab({ client }: { client: LinkMintClient }) {
  const roles = useAuthStore((s) => s.user?.roles ?? []);
  const {
    orgs,
    create,
    createError,
    selectedOrgId,
    members,
    membersLoading,
    membersError,
    loadMembers,
    addMember,
    removeMember,
  } = useOrganizations(client, roles);

  const [createOpen, setCreateOpen] = useState(false);
  const [orgName, setOrgName] = useState('');
  const [orgType, setOrgType] = useState<OrgType>('merchant');
  const [creating, setCreating] = useState(false);

  const [addOpen, setAddOpen] = useState(false);
  const [memberEmail, setMemberEmail] = useState('');
  const [memberRole, setMemberRole] = useState<Role>('viewer');
  const [adding, setAdding] = useState(false);

  const [removeTarget, setRemoveTarget] = useState<Member | null>(null);

  const columns = useMemo(() => memberColumns(setRemoveTarget), []);
  const selectedOrg = orgs.find((o) => o.org_id === selectedOrgId) ?? null;
  const hasMembers = members.length > 0;

  async function submitCreate(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (orgName.trim().length === 0) {
      return;
    }
    setCreating(true);
    const ok = await create({ name: orgName.trim(), type: orgType });
    setCreating(false);
    if (ok) {
      setCreateOpen(false);
      setOrgName('');
      setOrgType('merchant');
    }
  }

  async function submitAddMember(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!selectedOrgId || memberEmail.trim().length === 0) {
      return;
    }
    setAdding(true);
    const ok = await addMember(selectedOrgId, { email: memberEmail.trim(), role: memberRole });
    setAdding(false);
    if (ok) {
      setAddOpen(false);
      setMemberEmail('');
      setMemberRole('viewer');
    }
  }

  return (
    <Stack gap={6} pt={2}>
      <Panel>
        <Stack gap={4}>
          <HStack justify="space-between">
            <Box>
              <Text fontFamily="heading" fontWeight="600" fontSize="lg">
                Organizations
              </Text>
              <Text fontSize="sm" color="fg.muted">
                Workspaces you belong to. Members and roles are managed per org.
              </Text>
            </Box>
            <Button colorPalette="emerald" size="sm" gap={2} onClick={() => setCreateOpen(true)}>
              <Plus size={15} /> New organization
            </Button>
          </HStack>

          {orgs.length === 0 ? (
            <Text fontSize="sm" color="fg.muted">
              You don&apos;t belong to any organization yet. Create one to start issuing API keys
              and inviting members.
            </Text>
          ) : (
            <Stack gap={2}>
              {orgs.map((org) => {
                const active = org.org_id === selectedOrgId;
                return (
                  <HStack
                    key={org.org_id}
                    justify="space-between"
                    px={4}
                    py={3}
                    borderWidth="1px"
                    borderColor={active ? 'accent.solid' : 'border'}
                    borderRadius="md"
                    bg={active ? 'accent.subtle' : 'bg.panel'}
                  >
                    <Stack gap={0.5}>
                      <Text fontSize="sm" fontWeight="500">
                        {orgLabel(org)}
                      </Text>
                      <Text fontSize="xs" color="fg.muted" fontFamily="mono">
                        {org.org_id}
                      </Text>
                    </Stack>
                    <HStack gap={3}>
                      <Badge colorPalette="gray" variant="subtle" textTransform="capitalize">
                        {org.role}
                      </Badge>
                      <Button
                        size="xs"
                        variant="outline"
                        gap={1.5}
                        onClick={() => loadMembers(org.org_id)}
                      >
                        <Users size={13} /> Members
                      </Button>
                    </HStack>
                  </HStack>
                );
              })}
            </Stack>
          )}
        </Stack>
      </Panel>

      {selectedOrg ? (
        <Panel p={0} overflow="hidden">
          <HStack
            justify="space-between"
            px={6}
            py={4}
            borderBottomWidth="1px"
            borderColor="border"
          >
            <Text fontFamily="heading" fontWeight="600" fontSize="lg">
              Members · {orgLabel(selectedOrg)}
            </Text>
            <Button colorPalette="emerald" size="sm" gap={2} onClick={() => setAddOpen(true)}>
              <Plus size={15} /> Add member
            </Button>
          </HStack>

          {membersError ? (
            <Box px={6} pt={4}>
              <ErrorBanner error={membersError} />
            </Box>
          ) : null}

          <Loadable
            loading={membersLoading && !hasMembers}
            error={membersError}
            isEmpty={!hasMembers}
            hasData={hasMembers}
            skeleton={<TableSkeleton rows={3} label="Members table" />}
            empty={
              <Box p={6}>
                <Text fontSize="sm" color="fg.muted">
                  No members yet.
                </Text>
              </Box>
            }
          >
            <DataTable
              columns={columns}
              rows={members}
              rowKey={(m) => m.user_id}
              caption="Organization members"
            />
          </Loadable>
        </Panel>
      ) : null}

      {/* Create org modal */}
      <Modal
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        title="Create organization"
        footer={
          <>
            <Button variant="ghost" size="sm" onClick={() => setCreateOpen(false)}>
              Cancel
            </Button>
            <Button
              type="submit"
              form="create-org-form"
              colorPalette="emerald"
              size="sm"
              loading={creating}
              loadingText="Creating…"
              disabled={orgName.trim().length === 0}
            >
              Create
            </Button>
          </>
        }
      >
        <form id="create-org-form" onSubmit={submitCreate} noValidate>
          <Stack gap={4}>
            {createError ? <ErrorBanner error={createError} /> : null}
            <FormField label="Name" required>
              <Input
                value={orgName}
                onChange={(e) => setOrgName(e.target.value)}
                placeholder="Acme Inc."
                maxLength={120}
              />
            </FormField>
            <FormField label="Type" required>
              <NativeSelect.Root>
                <NativeSelect.Field
                  value={orgType}
                  onChange={(e) => setOrgType(e.target.value as OrgType)}
                >
                  {ORG_TYPES.map((t) => (
                    <option key={t} value={t}>
                      {t}
                    </option>
                  ))}
                </NativeSelect.Field>
                <NativeSelect.Indicator />
              </NativeSelect.Root>
            </FormField>
          </Stack>
        </form>
      </Modal>

      {/* Add member modal */}
      <Modal
        open={addOpen}
        onClose={() => setAddOpen(false)}
        title="Add member"
        description="Invite by email and assign a role."
        footer={
          <>
            <Button variant="ghost" size="sm" onClick={() => setAddOpen(false)}>
              Cancel
            </Button>
            <Button
              type="submit"
              form="add-member-form"
              colorPalette="emerald"
              size="sm"
              loading={adding}
              loadingText="Adding…"
              disabled={memberEmail.trim().length === 0}
            >
              Add member
            </Button>
          </>
        }
      >
        <form id="add-member-form" onSubmit={submitAddMember} noValidate>
          <Stack gap={4}>
            <FormField label="Email" required>
              <Input
                value={memberEmail}
                onChange={(e) => setMemberEmail(e.target.value)}
                type="email"
                placeholder="teammate@example.com"
              />
            </FormField>
            <FormField label="Role" required>
              <NativeSelect.Root>
                <NativeSelect.Field
                  value={memberRole}
                  onChange={(e) => setMemberRole(e.target.value as Role)}
                >
                  {ROLES.map((r) => (
                    <option key={r} value={r}>
                      {r}
                    </option>
                  ))}
                </NativeSelect.Field>
                <NativeSelect.Indicator />
              </NativeSelect.Root>
            </FormField>
          </Stack>
        </form>
      </Modal>

      {/* Remove member confirm */}
      <Modal
        open={removeTarget !== null}
        onClose={() => setRemoveTarget(null)}
        role="alertdialog"
        size="sm"
        title="Remove this member?"
        description="They will lose access to this organization."
        footer={
          <>
            <Button variant="ghost" size="sm" onClick={() => setRemoveTarget(null)}>
              Cancel
            </Button>
            <Button
              variant="solid"
              colorPalette="red"
              size="sm"
              onClick={() => {
                if (removeTarget && selectedOrgId) {
                  void removeMember(selectedOrgId, removeTarget.user_id);
                  setRemoveTarget(null);
                }
              }}
            >
              Remove
            </Button>
          </>
        }
      >
        {removeTarget ? (
          <Text fontSize="sm" color="fg.muted" fontFamily="mono">
            {removeTarget.user_id}
          </Text>
        ) : null}
      </Modal>
    </Stack>
  );
}
