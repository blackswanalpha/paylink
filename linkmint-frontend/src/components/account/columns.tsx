'use client';

/**
 * Shared DataTable column definitions for the Account tables — active sessions, API keys, and org
 * members. Presentational only: the trailing action menus delegate to the caller (which owns the
 * confirm modal + the SDK mutation), mirroring the PayLink columns pattern.
 */

import { Badge, HStack, Text } from '@chakra-ui/react';
import { MoreHorizontal, Trash2, XCircle } from 'react-feather';
import type { ApiKey, Member, Session } from '@linkmint/sdk';

import {
  AddressChip,
  IconButton,
  Menu,
  StatusPill,
  type DataTableColumn,
  type StatusKind,
} from '@/components/ui';

/** Short, locale-aware date for table cells (shared with the PayLink columns idiom). */
function formatShortDate(iso: string): string {
  const t = Date.parse(iso);
  if (Number.isNaN(t)) {
    return '—';
  }
  return new Date(t).toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

function apiKeyStatusKind(status: string): StatusKind {
  switch (status.toUpperCase()) {
    case 'ACTIVE':
      return 'success';
    case 'REVOKED':
      return 'danger';
    default:
      return 'neutral';
  }
}

// ── Sessions ──────────────────────────────────────────────────────────────────────────────────

export function sessionColumns(onRevoke: (s: Session) => void): DataTableColumn<Session>[] {
  return [
    {
      key: 'device',
      header: 'Device',
      render: (s) => (
        <HStack gap={2}>
          <Text fontSize="sm" maxW="280px" truncate title={s.user_agent ?? undefined}>
            {s.user_agent ?? 'Unknown device'}
          </Text>
          {s.current ? (
            <Badge colorPalette="emerald" variant="subtle" size="sm">
              This device
            </Badge>
          ) : null}
        </HStack>
      ),
    },
    {
      key: 'ip',
      header: 'IP',
      render: (s) => (
        <Text fontSize="sm" fontFamily="mono" color="fg.muted">
          {s.ip ?? '—'}
        </Text>
      ),
    },
    {
      key: 'created',
      header: 'Signed in',
      sortable: true,
      sortValue: (s) => Date.parse(s.created_at),
      render: (s) => (
        <Text fontSize="sm" color="fg.muted" whiteSpace="nowrap">
          {formatShortDate(s.created_at)}
        </Text>
      ),
    },
    {
      key: 'expires',
      header: 'Expires',
      render: (s) => (
        <Text fontSize="sm" color="fg.muted" whiteSpace="nowrap">
          {formatShortDate(s.expires_at)}
        </Text>
      ),
    },
    {
      key: 'actions',
      header: '',
      align: 'end',
      width: '48px',
      render: (s) => (
        <Menu
          placement="bottom-end"
          trigger={
            <IconButton
              aria-label={`Actions for session ${s.session_id}`}
              variant="ghost"
              size="sm"
            >
              <MoreHorizontal size={16} />
            </IconButton>
          }
          items={[
            {
              value: 'revoke',
              label: s.current ? 'Sign out this device' : 'Revoke session',
              icon: <XCircle size={14} />,
              tone: 'danger',
            },
          ]}
          onSelect={(value) => {
            if (value === 'revoke') {
              onRevoke(s);
            }
          }}
        />
      ),
    },
  ];
}

// ── API keys ──────────────────────────────────────────────────────────────────────────────────

export function apiKeyColumns(onRevoke: (k: ApiKey) => void): DataTableColumn<ApiKey>[] {
  return [
    {
      key: 'name',
      header: 'Name',
      render: (k) => <Text fontSize="sm">{k.name}</Text>,
    },
    {
      key: 'prefix',
      header: 'Key',
      render: (k) => (
        <Text fontSize="sm" fontFamily="mono" color="fg.muted">
          {k.prefix}…
        </Text>
      ),
    },
    {
      key: 'scopes',
      header: 'Scopes',
      render: (k) => (
        <Text fontSize="xs" color="fg.muted">
          {k.scopes.length > 0 ? k.scopes.join(', ') : 'none'}
        </Text>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      render: (k) => <StatusPill kind={apiKeyStatusKind(k.status)} label={k.status} />,
    },
    {
      key: 'created',
      header: 'Created',
      sortable: true,
      sortValue: (k) => Date.parse(k.created_at),
      render: (k) => (
        <Text fontSize="sm" color="fg.muted" whiteSpace="nowrap">
          {formatShortDate(k.created_at)}
        </Text>
      ),
    },
    {
      key: 'actions',
      header: '',
      align: 'end',
      width: '48px',
      render: (k) =>
        k.status.toUpperCase() === 'ACTIVE' ? (
          <Menu
            placement="bottom-end"
            trigger={
              <IconButton aria-label={`Actions for API key ${k.name}`} variant="ghost" size="sm">
                <MoreHorizontal size={16} />
              </IconButton>
            }
            items={[
              { value: 'revoke', label: 'Revoke key', icon: <XCircle size={14} />, tone: 'danger' },
            ]}
            onSelect={(value) => {
              if (value === 'revoke') {
                onRevoke(k);
              }
            }}
          />
        ) : null,
    },
  ];
}

// ── Org members ───────────────────────────────────────────────────────────────────────────────

export function memberColumns(onRemove: (m: Member) => void): DataTableColumn<Member>[] {
  return [
    {
      key: 'user',
      header: 'Member',
      render: (m) => <AddressChip value={m.user_id} label="user id" />,
    },
    {
      key: 'role',
      header: 'Role',
      render: (m) => (
        <Badge colorPalette="gray" variant="subtle" textTransform="capitalize">
          {m.role}
        </Badge>
      ),
    },
    {
      key: 'actions',
      header: '',
      align: 'end',
      width: '48px',
      render: (m) => (
        <Menu
          placement="bottom-end"
          trigger={
            <IconButton aria-label={`Actions for member ${m.user_id}`} variant="ghost" size="sm">
              <MoreHorizontal size={16} />
            </IconButton>
          }
          items={[
            { value: 'remove', label: 'Remove member', icon: <Trash2 size={14} />, tone: 'danger' },
          ]}
          onSelect={(value) => {
            if (value === 'remove') {
              onRemove(m);
            }
          }}
        />
      ),
    },
  ];
}
