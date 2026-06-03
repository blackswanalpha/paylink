/**
 * useSessions — optimistic revoke removes the row on success and rolls it back when the mutation fails.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { ConflictError, type LinkMintClient, type Session } from '@linkmint/sdk';

import { useSessions } from '@/hooks/useSessions';

const SESSIONS: Session[] = [
  {
    session_id: 's1',
    user_agent: 'Chrome',
    ip: '1.2.3.4',
    created_at: '2026-01-01T00:00:00Z',
    expires_at: '2026-02-01T00:00:00Z',
    current: false,
  },
  {
    session_id: 's2',
    user_agent: 'Firefox',
    ip: null,
    created_at: '2026-01-02T00:00:00Z',
    expires_at: '2026-02-02T00:00:00Z',
    current: true,
  },
];

function makeClient(revoke: ReturnType<typeof vi.fn>): LinkMintClient {
  return {
    sessions: {
      list: vi.fn().mockResolvedValue({ items: SESSIONS }),
      revoke,
    },
  } as unknown as LinkMintClient;
}

beforeEach(() => {
  vi.restoreAllMocks();
});

describe('useSessions', () => {
  it('removes the row on a successful revoke', async () => {
    const client = makeClient(vi.fn().mockResolvedValue({ status: 'revoked', session_id: 's1' }));
    const { result } = renderHook(() => useSessions(client));
    await waitFor(() => expect(result.current.items).toHaveLength(2));

    await act(async () => {
      await result.current.revoke('s1');
    });

    expect(result.current.items.map((s) => s.session_id)).toEqual(['s2']);
  });

  it('rolls back the removal when revoke rejects', async () => {
    const client = makeClient(
      vi.fn().mockRejectedValue(
        new ConflictError({
          status: 409,
          code: 'IDEMPOTENT_CONFLICT',
          message: 'conflict',
          details: {},
          traceId: undefined,
          requestId: undefined,
        }),
      ),
    );
    const { result } = renderHook(() => useSessions(client));
    await waitFor(() => expect(result.current.items).toHaveLength(2));

    await act(async () => {
      await result.current.revoke('s1');
    });

    expect(result.current.items).toHaveLength(2); // restored
  });
});
