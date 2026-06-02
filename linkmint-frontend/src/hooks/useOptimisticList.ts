'use client';

/**
 * useOptimisticList (work06) — generic mutate-then-reconcile over an in-memory list, with rollback.
 *
 * The owner hook keeps the list state (e.g. usePayLinks' `items`); this helper is handed the current
 * items + setter + a key fn and runs ONE optimistic mutation against a matched item:
 *
 *   snapshot  → capture the matched item for rollback
 *   apply     → optimistically transform the matched item in place (status flip, …) — pure, no I/O
 *   commit    → run the async SDK call
 *   reconcile → replace the item with the authoritative server value (from the result / a follow-up get)
 *   rollback  → on error, restore the snapshot, then `onError` (route via reportError there)
 *
 * Reusable across mutations: cancel a PayLink (apply: status→CANCELLED), revoke a key, etc. The helper
 * is single-flight per key. `apply` must preserve the item's key (identity is keyed via `keyOf`).
 *
 * Note: `pending` is a ref-backed Set (a single-flight guard) and is NOT reactive — for work06's
 * optimistic cancel the instant flip IS the feedback, so a reactive per-row spinner isn't needed. A
 * future consumer that needs reactive pending state should lift the set into `useState`.
 */

import { useCallback, useRef } from 'react';

export interface OptimisticRunArgs<T, R> {
  /** Stable identity predicate for the target item (e.g. `(pl) => pl.pl_id === plId`). */
  match: (item: T) => boolean;
  /** Optimistic transform of the matched item; return a NEW item (must keep the same key). */
  apply: (item: T) => T;
  /** The async mutation (the SDK call). Receives the snapshot of the matched item. */
  commit: (snapshot: T) => Promise<R>;
  /** Map the commit result (+ snapshot) to the reconciled item. Omit to keep the applied value. */
  reconcile?: (result: R, snapshot: T) => T | Promise<T>;
  /** Called on failure AFTER rollback. Route via reportError here. */
  onError?: (err: unknown, snapshot: T) => void;
}

export interface UseOptimisticListApi<T> {
  /** Run one optimistic mutation. Resolves true on success, false on error / no-op. */
  run: <R>(args: OptimisticRunArgs<T, R>) => Promise<boolean>;
  /** Keys with an in-flight mutation (ref-backed single-flight guard; non-reactive — see note). */
  pending: ReadonlySet<string>;
}

export function useOptimisticList<T>(
  items: T[],
  setItems: (updater: (prev: T[]) => T[]) => void,
  keyOf: (item: T) => string,
): UseOptimisticListApi<T> {
  const inFlight = useRef<Set<string>>(new Set());

  const run = useCallback(
    async <R>(args: OptimisticRunArgs<T, R>): Promise<boolean> => {
      const { match, apply, commit, reconcile, onError } = args;

      const snapshot = items.find(match);
      if (snapshot === undefined) {
        return false; // nothing to mutate (already gone / filtered out)
      }
      const key = keyOf(snapshot);
      if (inFlight.current.has(key)) {
        return false; // single-flight per item
      }
      inFlight.current.add(key);

      // apply — optimistic flip
      setItems((prev) => prev.map((it) => (match(it) ? apply(it) : it)));
      try {
        const result = await commit(snapshot);
        if (reconcile) {
          const reconciled = await reconcile(result, snapshot);
          setItems((prev) => prev.map((it) => (keyOf(it) === key ? reconciled : it)));
        }
        return true;
      } catch (err) {
        // rollback — restore the pre-mutation snapshot for this item
        setItems((prev) => prev.map((it) => (keyOf(it) === key ? snapshot : it)));
        onError?.(err, snapshot);
        return false;
      } finally {
        inFlight.current.delete(key);
      }
    },
    [items, setItems, keyOf],
  );

  return { run, pending: inFlight.current };
}
