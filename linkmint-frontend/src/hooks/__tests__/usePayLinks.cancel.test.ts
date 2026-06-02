/**
 * usePayLinks.cancel — the optimistic cancel wired to the SDK. Asserts the row flips to CANCELLED
 * immediately, the cancel + reconcile (get) fire, aggregates update, a rejected cancel rolls back and
 * surfaces an inline error (without reading), and a failed reconcile-read still trusts the cancel
 * result's status.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, act, waitFor } from '@/test/renderWithTheme';
import { createApiError, type LinkMintClient, type PayLink } from '@linkmint/sdk';
import { usePayLinks } from '@/hooks/usePayLinks';
import { useErrorStore } from '@/store/errors';

vi.mock('sonner', () => ({ toast: { error: vi.fn() } }));

function payLink(overrides: Partial<PayLink> = {}): PayLink {
  return {
    pl_id: '0xpl_a',
    creator: '0xcreator',
    receiver: '0xreceiver',
    owner: '0xcreator',
    amount: 10000,
    currency: 'KES',
    status: 'CREATED',
    expiry: '2099-01-01T00:00:00Z',
    usage: 'single',
    vote_count: 0,
    chain_tx_hash: null,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    verified_at: null,
    ...overrides,
  };
}

interface MockPayLinks {
  list: ReturnType<typeof vi.fn>;
  cancel: ReturnType<typeof vi.fn>;
  get: ReturnType<typeof vi.fn>;
}

function mockClient(paylinks: MockPayLinks): LinkMintClient {
  return { paylinks } as unknown as LinkMintClient;
}

beforeEach(() => {
  useErrorStore.setState({ reauth: null, kyc: null });
});

describe('usePayLinks cancel', () => {
  it('optimistically cancels, reconciles via get, and updates aggregates', async () => {
    const paylinks: MockPayLinks = {
      list: vi.fn().mockResolvedValue({ items: [payLink({ status: 'CREATED' })] }),
      cancel: vi.fn().mockResolvedValue({ pl_id: '0xpl_a', status: 'CANCELLED' }),
      get: vi.fn().mockResolvedValue(payLink({ status: 'CANCELLED' })),
    };
    const client = mockClient(paylinks);
    const { result } = renderHook(() => usePayLinks(client, '0xcreator'));
    await waitFor(() => expect(result.current.items).toHaveLength(1));
    expect(result.current.aggregates.activeCount).toBe(1);

    await act(async () => {
      await result.current.cancel('0xpl_a');
    });

    expect(paylinks.cancel).toHaveBeenCalledWith('0xpl_a');
    expect(paylinks.get).toHaveBeenCalledWith('0xpl_a');
    expect(result.current.items[0]?.status).toBe('CANCELLED');
    expect(result.current.aggregates.activeCount).toBe(0);
  });

  it('shows the optimistic flip before the cancel resolves', async () => {
    let resolveCancel: (v: { pl_id: string; status: string }) => void = () => undefined;
    const paylinks: MockPayLinks = {
      list: vi.fn().mockResolvedValue({ items: [payLink({ status: 'CREATED' })] }),
      cancel: vi.fn().mockImplementation(() => new Promise((r) => (resolveCancel = r))),
      get: vi.fn().mockResolvedValue(payLink({ status: 'CANCELLED' })),
    };
    const client = mockClient(paylinks);
    const { result } = renderHook(() => usePayLinks(client, '0xcreator'));
    await waitFor(() => expect(result.current.items).toHaveLength(1));

    let pending: Promise<boolean> = Promise.resolve(false);
    act(() => {
      pending = result.current.cancel('0xpl_a');
    });
    expect(result.current.items[0]?.status).toBe('CANCELLED'); // optimistic, before the SDK resolves

    await act(async () => {
      resolveCancel({ pl_id: '0xpl_a', status: 'CANCELLED' });
      await pending;
    });
  });

  it('rolls back and surfaces an inline error when cancel fails', async () => {
    const paylinks: MockPayLinks = {
      list: vi.fn().mockResolvedValue({ items: [payLink({ status: 'CREATED' })] }),
      cancel: vi.fn().mockRejectedValue(
        createApiError({
          status: 409,
          code: 'PAYLINK_NOT_PAYABLE',
          message: 'not payable',
          details: {},
          traceId: undefined,
          requestId: undefined,
        }),
      ),
      get: vi.fn(),
    };
    const client = mockClient(paylinks);
    const { result } = renderHook(() => usePayLinks(client, '0xcreator'));
    await waitFor(() => expect(result.current.items).toHaveLength(1));

    await act(async () => {
      await result.current.cancel('0xpl_a');
    });

    expect(result.current.items[0]?.status).toBe('CREATED'); // rolled back
    expect(result.current.error).not.toBeNull(); // inline error surfaced
    expect(paylinks.get).not.toHaveBeenCalled();
  });

  it('trusts the cancel result status when the reconcile read fails', async () => {
    const paylinks: MockPayLinks = {
      list: vi.fn().mockResolvedValue({ items: [payLink({ status: 'CREATED' })] }),
      cancel: vi.fn().mockResolvedValue({ pl_id: '0xpl_a', status: 'CANCELLED' }),
      get: vi.fn().mockRejectedValue(new Error('network')),
    };
    const client = mockClient(paylinks);
    const { result } = renderHook(() => usePayLinks(client, '0xcreator'));
    await waitFor(() => expect(result.current.items).toHaveLength(1));

    await act(async () => {
      await result.current.cancel('0xpl_a');
    });

    expect(result.current.items[0]?.status).toBe('CANCELLED'); // from the cancel result, despite get failing
  });
});
