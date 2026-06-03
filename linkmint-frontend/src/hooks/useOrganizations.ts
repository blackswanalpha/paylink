'use client';

/**
 * useOrganizations — the user's orgs + per-org member management.
 *
 * The initial org list renders immediately from `UserProfile.roles` (org_id + role; names unknown),
 * then `client.organizations.list()` replaces it with the server truth — real names that survive a
 * refresh. If that call fails, the roles-derived fallback stays (names are an enhancement, reported
 * silently). Creating an org returns its name and prepends it. Selecting an org loads its members;
 * add is appended, remove is optimistic (filter out, rollback on error). `CANNOT_REMOVE_LAST_OWNER`
 * surfaces inline.
 */

import { useCallback, useEffect, useState } from 'react';
import type {
  AddMemberInput,
  CreateOrgInput,
  LinkMintClient,
  Member,
  OrgRoleEntry,
} from '@linkmint/sdk';

import { isAbortError, type DisplayError } from '@/lib/errors';
import { reportError } from '@/lib/reportError';

export interface OrgEntry {
  org_id: string;
  /** Known only for orgs created in this session; null for those derived from roles. */
  name: string | null;
  role: string;
}

export interface UseOrganizationsResult {
  orgs: OrgEntry[];
  createError: DisplayError | null;
  create: (input: CreateOrgInput) => Promise<boolean>;
  selectedOrgId: string | null;
  members: Member[];
  membersLoading: boolean;
  membersError: DisplayError | null;
  loadMembers: (orgId: string) => void;
  addMember: (orgId: string, input: AddMemberInput) => Promise<boolean>;
  removeMember: (orgId: string, userId: string) => Promise<boolean>;
}

export function useOrganizations(
  client: LinkMintClient | null,
  initialRoles: OrgRoleEntry[],
): UseOrganizationsResult {
  const [orgs, setOrgs] = useState<OrgEntry[]>(() =>
    initialRoles.map((r) => ({ org_id: r.org_id, name: null, role: r.role })),
  );
  const [createError, setCreateError] = useState<DisplayError | null>(null);
  const [selectedOrgId, setSelectedOrgId] = useState<string | null>(null);
  const [members, setMembers] = useState<Member[]>([]);
  const [membersLoading, setMembersLoading] = useState(false);
  const [membersError, setMembersError] = useState<DisplayError | null>(null);

  // Replace the roles-derived list with the server's (names included) once it loads.
  useEffect(() => {
    if (!client) {
      return;
    }
    const controller = new AbortController();
    client.organizations
      .list({ signal: controller.signal })
      .then((res) => {
        setOrgs(res.items.map((o) => ({ org_id: o.org_id, name: o.name, role: o.role })));
      })
      .catch((err: unknown) => {
        if (isAbortError(err)) {
          return;
        }
        // Names are an enhancement; keep the roles-derived fallback and don't hijack a surface.
        reportError(err, { silent: true });
      });
    return () => controller.abort();
  }, [client]);

  const create = useCallback(
    async (input: CreateOrgInput): Promise<boolean> => {
      if (!client) {
        return false;
      }
      setCreateError(null);
      try {
        const org = await client.organizations.create(input);
        setOrgs((prev) => [{ org_id: org.org_id, name: org.name, role: org.role }, ...prev]);
        return true;
      } catch (err) {
        const { error, surface } = reportError(err, { surface: 'inline' });
        if (surface === 'inline') {
          setCreateError(error);
        }
        return false;
      }
    },
    [client],
  );

  const loadMembers = useCallback(
    (orgId: string) => {
      if (!client) {
        return;
      }
      setSelectedOrgId(orgId);
      setMembersLoading(true);
      setMembersError(null);
      setMembers([]);
      void client.organizations
        .listMembers(orgId)
        .then((res) => setMembers(res.items))
        .catch((err) => {
          const { error, surface } = reportError(err, { surface: 'inline' });
          if (surface === 'inline') {
            setMembersError(error);
          }
        })
        .finally(() => setMembersLoading(false));
    },
    [client],
  );

  const addMember = useCallback(
    async (orgId: string, input: AddMemberInput): Promise<boolean> => {
      if (!client) {
        return false;
      }
      setMembersError(null);
      try {
        const member = await client.organizations.addMember(orgId, input);
        setMembers((prev) => [...prev, member]);
        return true;
      } catch (err) {
        const { error, surface } = reportError(err, { surface: 'inline' });
        if (surface === 'inline') {
          setMembersError(error);
        }
        return false;
      }
    },
    [client],
  );

  const removeMember = useCallback(
    async (orgId: string, userId: string): Promise<boolean> => {
      if (!client) {
        return false;
      }
      const removed = members.find((m) => m.user_id === userId);
      setMembers((prev) => prev.filter((m) => m.user_id !== userId));
      try {
        await client.organizations.removeMember(orgId, userId);
        return true;
      } catch (err) {
        // Functional rollback (e.g. CANNOT_REMOVE_LAST_OWNER): re-insert only the removed member.
        if (removed) {
          setMembers((prev) =>
            prev.some((m) => m.user_id === userId) ? prev : [...prev, removed],
          );
        }
        const { error, surface } = reportError(err, { surface: 'inline' });
        if (surface === 'inline') {
          setMembersError(error);
        }
        return false;
      }
    },
    [client, members],
  );

  return {
    orgs,
    createError,
    create,
    selectedOrgId,
    members,
    membersLoading,
    membersError,
    loadMembers,
    addMember,
    removeMember,
  };
}
