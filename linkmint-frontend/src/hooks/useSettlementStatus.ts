'use client';

/**
 * Poll the PayLink until it reaches a terminal status, surfacing vote_count / chain_tx_hash /
 * verified_at for the status view. We poll the PayLink (not the Payment) because settlement is
 * finalized on-chain and projected onto the PayLink — and, per work35, a Payment may not exist.
 *
 * Cleanup aborts the in-flight request and clears the timer; transient poll errors don't stop the
 * loop (settlement is async), but a 404 does (the id is wrong).
 */

import { useEffect, useState } from 'react';
import { toast } from 'sonner';
import { NotFoundError } from '@linkmint/sdk';
import type { PayLink, PayLinkStatus } from '@linkmint/sdk';
import { useAppStore } from '@/store/app';
import { clientConfig } from '@/lib/env';
import { isAbortError, toDisplayError, type DisplayError } from '@/lib/errors';

const TERMINAL: ReadonlySet<PayLinkStatus> = new Set([
  'VERIFIED',
  'FAILED',
  'CANCELLED',
  'EXPIRED',
]);

export interface SettlementState {
  paylink: PayLink | null;
  status: PayLinkStatus | null;
  isPolling: boolean;
  isTerminal: boolean;
  error: DisplayError | null;
}

export function useSettlementStatus(plId: string): SettlementState {
  const client = useAppStore((s) => s.client);
  const [state, setState] = useState<SettlementState>({
    paylink: null,
    status: null,
    isPolling: true,
    isTerminal: false,
    error: null,
  });

  useEffect(() => {
    const c = client;
    if (!c) return;
    const pollMs = clientConfig().pollMs;
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | null = null;
    const controller = new AbortController();

    const finishToast = (status: PayLinkStatus): void => {
      if (status === 'VERIFIED') {
        toast.success('Settled on-chain', { description: 'The PayLink is VERIFIED.' });
      } else {
        toast.error(`PayLink ${status.toLowerCase()}`, {
          description: 'Settlement did not complete.',
        });
      }
    };

    const tick = async (): Promise<void> => {
      if (cancelled) return;
      try {
        const pl = await c.paylinks.get(plId, { signal: controller.signal });
        if (cancelled) return;
        const terminal = TERMINAL.has(pl.status);
        setState({
          paylink: pl,
          status: pl.status,
          isPolling: !terminal,
          isTerminal: terminal,
          error: null,
        });
        if (terminal) {
          finishToast(pl.status);
          return;
        }
      } catch (err) {
        if (cancelled || isAbortError(err)) return;
        if (err instanceof NotFoundError) {
          setState((prev) => ({ ...prev, isPolling: false, error: toDisplayError(err) }));
          return;
        }
        setState((prev) => ({ ...prev, error: toDisplayError(err) }));
      }
      if (!cancelled) {
        timer = setTimeout(() => void tick(), pollMs);
      }
    };

    void tick();

    return () => {
      cancelled = true;
      controller.abort();
      if (timer) clearTimeout(timer);
    };
  }, [client, plId]);

  return state;
}
