'use client';

/**
 * usePayLinks — loads the merchant's PayLinks via the SDK (`paylinks.list`, LIVE/work01) and derives
 * the dashboard aggregates client-side (counts by status, total settled, a recent-activity series).
 * Richer analytics (true revenue series, conversion) are PLANNED on the reporting service (work26).
 *
 * Errors are routed through the work04 error system (`reportError`, F.5) — surfaced inline with a
 * retry, while a 401 escalates to the global re-auth modal. The fetch is abortable and guards against
 * post-unmount state updates.
 */

import { useCallback, useEffect, useMemo, useState } from 'react';
import type { LinkMintClient, PayLink, PayLinkStatus } from '@linkmint/sdk';
import { isAbortError, type DisplayError } from '@/lib/errors';
import { reportError } from '@/lib/reportError';

export interface PayLinkAggregates {
  total: number;
  byStatus: Record<PayLinkStatus, number>;
  totalSettledMinor: number;
  activeCount: number;
  pendingCount: number;
  verifiedCount: number;
  /** Dominant currency for headline display (first seen), or null when empty. */
  currency: string | null;
  /** Recent-activity series: PayLinks created per day over the last 8 days. */
  sparkline: number[];
}

const EMPTY_BY_STATUS: Record<PayLinkStatus, number> = {
  CREATED: 0,
  PENDING: 0,
  VERIFIED: 0,
  FAILED: 0,
  CANCELLED: 0,
  EXPIRED: 0,
};

function buildSparkline(items: PayLink[], now: number): number[] {
  const days = 8;
  const dayMs = 86_400_000;
  const buckets = new Array<number>(days).fill(0);
  for (const pl of items) {
    const t = Date.parse(pl.created_at);
    if (Number.isNaN(t)) {
      continue;
    }
    const ageDays = Math.floor((now - t) / dayMs);
    if (ageDays >= 0 && ageDays < days) {
      const idx = days - 1 - ageDays;
      buckets[idx] = (buckets[idx] ?? 0) + 1;
    }
  }
  return buckets;
}

function computeAggregates(items: PayLink[], now: number): PayLinkAggregates {
  const byStatus: Record<PayLinkStatus, number> = { ...EMPTY_BY_STATUS };
  let totalSettledMinor = 0;
  let currency: string | null = null;
  for (const pl of items) {
    byStatus[pl.status] += 1;
    if (pl.status === 'VERIFIED') {
      totalSettledMinor += pl.amount;
    }
    if (currency === null) {
      currency = pl.currency;
    }
  }
  return {
    total: items.length,
    byStatus,
    totalSettledMinor,
    activeCount: byStatus.CREATED + byStatus.PENDING,
    pendingCount: byStatus.PENDING,
    verifiedCount: byStatus.VERIFIED,
    currency,
    sparkline: buildSparkline(items, now),
  };
}

export interface UsePayLinksResult {
  items: PayLink[];
  aggregates: PayLinkAggregates;
  loading: boolean;
  error: DisplayError | null;
  refresh: () => void;
}

export function usePayLinks(client: LinkMintClient | null, creator?: string): UsePayLinksResult {
  const [items, setItems] = useState<PayLink[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<DisplayError | null>(null);

  const load = useCallback(
    async (signal: AbortSignal) => {
      if (!client) {
        return;
      }
      setLoading(true);
      setError(null);
      try {
        const res = await client.paylinks.list({ creator, limit: 100 }, { signal });
        if (!signal.aborted) {
          setItems(res.items);
        }
      } catch (err) {
        if (isAbortError(err) || signal.aborted) {
          return;
        }
        // Route through the system, inline (the dashboard renders the banner + a retry). A 401
        // escalates to the global re-auth modal instead of rendering inline.
        const { error, surface } = reportError(err, { surface: 'inline' });
        if (surface === 'inline') {
          setError(error);
        }
      } finally {
        if (!signal.aborted) {
          setLoading(false);
        }
      }
    },
    [client, creator],
  );

  useEffect(() => {
    if (!client) {
      return;
    }
    const controller = new AbortController();
    void load(controller.signal);
    return () => controller.abort();
  }, [client, load]);

  const refresh = useCallback(() => {
    const controller = new AbortController();
    void load(controller.signal);
  }, [load]);

  const aggregates = useMemo(() => computeAggregates(items, Date.now()), [items]);

  return { items, aggregates, loading, error, refresh };
}
