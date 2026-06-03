/**
 * useApiKeys — issuing returns the one-time `full_key` and inserts a secret-free list row; revoking
 * optimistically flips the row to REVOKED and rolls back on a rejected mutation.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import {
  ConflictError,
  type ApiKey,
  type IssueApiKeyResult,
  type LinkMintClient,
} from '@linkmint/sdk';

import { useApiKeys } from '@/hooks/useApiKeys';

function makeClient(over: Record<string, unknown>): LinkMintClient {
  return {
    users: {
      listApiKeys: vi.fn().mockResolvedValue({ items: [] }),
      createApiKey: vi.fn(),
      revokeApiKey: vi.fn(),
      ...over,
    },
  } as unknown as LinkMintClient;
}

const ACTIVE_KEY: ApiKey = {
  api_key_id: 'k1',
  org_id: 'o1',
  name: 'CI',
  prefix: 'lm_abc',
  scopes: [],
  status: 'ACTIVE',
  created_at: '2026-01-01T00:00:00Z',
  revoked_at: null,
};

function conflict(): ConflictError {
  return new ConflictError({
    status: 409,
    code: 'IDEMPOTENT_CONFLICT',
    message: 'conflict',
    details: {},
    traceId: undefined,
    requestId: undefined,
  });
}

beforeEach(() => {
  vi.restoreAllMocks();
});

describe('useApiKeys', () => {
  it('create returns the one-time full_key and inserts a secret-free row', async () => {
    const created = {
      api_key_id: 'k9',
      org_id: 'o1',
      name: 'CI',
      prefix: 'lm_xyz',
      full_key: 'lm_xyz_THE_SECRET',
      scopes: ['paylinks:read'],
      status: 'ACTIVE',
      created_at: '2026-02-01T00:00:00Z',
    };
    const client = makeClient({ createApiKey: vi.fn().mockResolvedValue(created) });
    const { result } = renderHook(() => useApiKeys(client));
    await waitFor(() => expect(result.current.loading).toBe(false));

    const captured: { value: IssueApiKeyResult | null } = { value: null };
    await act(async () => {
      captured.value = await result.current.create({ org_id: 'o1', name: 'CI' });
    });

    expect(captured.value?.full_key).toBe('lm_xyz_THE_SECRET');
    expect(result.current.items).toHaveLength(1);
    expect(result.current.items[0]?.api_key_id).toBe('k9');
    // The list row never carries the secret.
    expect(result.current.items[0]).not.toHaveProperty('full_key');
  });

  it('revoke flips to REVOKED on success', async () => {
    const client = makeClient({
      listApiKeys: vi.fn().mockResolvedValue({ items: [ACTIVE_KEY] }),
      revokeApiKey: vi.fn().mockResolvedValue({ api_key_id: 'k1', status: 'REVOKED' }),
    });
    const { result } = renderHook(() => useApiKeys(client));
    await waitFor(() => expect(result.current.items).toHaveLength(1));

    await act(async () => {
      await result.current.revoke('k1');
    });

    expect(result.current.items[0]?.status).toBe('REVOKED');
  });

  it('rolls back to ACTIVE if revoke rejects', async () => {
    const client = makeClient({
      listApiKeys: vi.fn().mockResolvedValue({ items: [ACTIVE_KEY] }),
      revokeApiKey: vi.fn().mockRejectedValue(conflict()),
    });
    const { result } = renderHook(() => useApiKeys(client));
    await waitFor(() => expect(result.current.items).toHaveLength(1));

    await act(async () => {
      await result.current.revoke('k1');
    });

    expect(result.current.items[0]?.status).toBe('ACTIVE'); // rolled back
  });
});
