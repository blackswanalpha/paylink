'use client';

/**
 * useApiKeys — loads the caller's API keys (`users.listApiKeys`), issues a new one
 * (`users.createApiKey`, returning the one-time `full_key` for the caller to reveal), and revokes one
 * (optimistic in-place flip to REVOKED, rollback on error). The list never carries secret material.
 */

import { useCallback, useEffect, useState } from 'react';
import type { ApiKey, IssueApiKeyInput, IssueApiKeyResult, LinkMintClient } from '@linkmint/sdk';

import { isAbortError, type DisplayError } from '@/lib/errors';
import { reportError } from '@/lib/reportError';

export interface UseApiKeysResult {
  items: ApiKey[];
  loading: boolean;
  error: DisplayError | null;
  refresh: () => void;
  /** Issue a key. Returns the full result (incl. the one-time `full_key`) or null on error. */
  create: (input: IssueApiKeyInput) => Promise<IssueApiKeyResult | null>;
  /** Optimistically revoke (flip to REVOKED); rolls back + reports inline on failure. */
  revoke: (apiKeyId: string) => Promise<boolean>;
}

export function useApiKeys(client: LinkMintClient | null): UseApiKeysResult {
  const [items, setItems] = useState<ApiKey[]>([]);
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
        const res = await client.users.listApiKeys({ signal });
        if (!signal.aborted) {
          setItems(res.items);
        }
      } catch (err) {
        if (isAbortError(err) || signal.aborted) {
          return;
        }
        const { error: reported, surface } = reportError(err, { surface: 'inline' });
        if (surface === 'inline') {
          setError(reported);
        }
      } finally {
        if (!signal.aborted) {
          setLoading(false);
        }
      }
    },
    [client],
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

  const create = useCallback(
    async (input: IssueApiKeyInput): Promise<IssueApiKeyResult | null> => {
      if (!client) {
        return null;
      }
      try {
        const result = await client.users.createApiKey(input);
        // Insert the listing row (no secret) so the new key shows immediately.
        const row: ApiKey = {
          api_key_id: result.api_key_id,
          org_id: result.org_id,
          name: result.name,
          prefix: result.prefix,
          scopes: result.scopes,
          status: result.status,
          created_at: result.created_at,
          revoked_at: null,
        };
        setItems((prev) => [row, ...prev]);
        return result;
      } catch (err) {
        const { error: reported, surface } = reportError(err, { surface: 'inline' });
        if (surface === 'inline') {
          setError(reported);
        }
        return null;
      }
    },
    [client],
  );

  const revoke = useCallback(
    async (apiKeyId: string): Promise<boolean> => {
      if (!client) {
        return false;
      }
      const original = items.find((k) => k.api_key_id === apiKeyId);
      setItems((prev) =>
        prev.map((k) =>
          k.api_key_id === apiKeyId
            ? { ...k, status: 'REVOKED', revoked_at: new Date().toISOString() }
            : k,
        ),
      );
      try {
        await client.users.revokeApiKey(apiKeyId);
        return true;
      } catch (err) {
        // Functional rollback: restore only this key, preserving any concurrent inserts.
        if (original) {
          setItems((prev) => prev.map((k) => (k.api_key_id === apiKeyId ? original : k)));
        }
        const { error: reported, surface } = reportError(err, { surface: 'inline' });
        if (surface === 'inline') {
          setError(reported);
        }
        return false;
      }
    },
    [client, items],
  );

  return { items, loading, error, refresh, create, revoke };
}
