/**
 * useOptimisticList — generic optimistic mutate-then-reconcile-with-rollback. Asserts the optimistic
 * apply is visible before commit resolves, reconcile replaces with the server value, a rejected commit
 * rolls back and calls onError, mutations are single-flight per key, and a match-miss is a no-op.
 */

import { useState } from 'react';
import { describe, it, expect, vi } from 'vitest';
import { renderHook, act } from '@/test/renderWithTheme';
import { useOptimisticList } from '@/hooks/useOptimisticList';

interface Item {
  id: string;
  status: string;
}

const INITIAL: Item[] = [
  { id: 'a', status: 'CREATED' },
  { id: 'b', status: 'PENDING' },
];

function useHarness(initial: Item[]) {
  const [items, setItems] = useState<Item[]>(initial);
  const optimistic = useOptimisticList<Item>(items, setItems, (it) => it.id);
  return { items, optimistic };
}

describe('useOptimisticList', () => {
  it('applies optimistically, then reconciles to the server value', async () => {
    const { result } = renderHook(() => useHarness(INITIAL));
    let ok = false;
    await act(async () => {
      ok = await result.current.optimistic.run<{ status: string }>({
        match: (it) => it.id === 'a',
        apply: (it) => ({ ...it, status: 'CANCELLED' }),
        commit: () => Promise.resolve({ status: 'CANCELLED' }),
        reconcile: (res, snap) => ({ ...snap, status: res.status }),
      });
    });
    expect(ok).toBe(true);
    expect(result.current.items.find((i) => i.id === 'a')?.status).toBe('CANCELLED');
    expect(result.current.items.find((i) => i.id === 'b')?.status).toBe('PENDING'); // untouched
  });

  it('shows the optimistic flip before the commit resolves', async () => {
    let resolveCommit: (v: { status: string }) => void = () => undefined;
    const { result } = renderHook(() => useHarness(INITIAL));

    let pending: Promise<boolean> = Promise.resolve(false);
    act(() => {
      pending = result.current.optimistic.run<{ status: string }>({
        match: (it) => it.id === 'a',
        apply: (it) => ({ ...it, status: 'CANCELLED' }),
        commit: () => new Promise((r) => (resolveCommit = r)),
      });
    });
    expect(result.current.items.find((i) => i.id === 'a')?.status).toBe('CANCELLED');

    await act(async () => {
      resolveCommit({ status: 'CANCELLED' });
      await pending;
    });
  });

  it('rolls back and calls onError when commit rejects', async () => {
    const onError = vi.fn();
    const { result } = renderHook(() => useHarness(INITIAL));
    let ok = true;
    await act(async () => {
      ok = await result.current.optimistic.run({
        match: (it) => it.id === 'a',
        apply: (it) => ({ ...it, status: 'CANCELLED' }),
        commit: () => Promise.reject(new Error('boom')),
        onError,
      });
    });
    expect(ok).toBe(false);
    expect(result.current.items.find((i) => i.id === 'a')?.status).toBe('CREATED'); // restored
    expect(onError).toHaveBeenCalledTimes(1);
  });

  it('is single-flight per key', async () => {
    let resolveCommit: (v: number) => void = () => undefined;
    const commit = vi
      .fn()
      .mockImplementation(() => new Promise<number>((r) => (resolveCommit = r)));
    const { result } = renderHook(() => useHarness(INITIAL));

    let first: Promise<boolean> = Promise.resolve(false);
    act(() => {
      first = result.current.optimistic.run<number>({
        match: (it) => it.id === 'a',
        apply: (it) => ({ ...it, status: 'CANCELLED' }),
        commit,
      });
    });

    let second = true;
    await act(async () => {
      second = await result.current.optimistic.run<number>({
        match: (it) => it.id === 'a',
        apply: (it) => ({ ...it, status: 'CANCELLED' }),
        commit,
      });
    });
    expect(second).toBe(false); // ignored while the first is in flight
    expect(commit).toHaveBeenCalledTimes(1);

    await act(async () => {
      resolveCommit(1);
      await first;
    });
  });

  it('is a no-op when nothing matches', async () => {
    const commit = vi.fn();
    const { result } = renderHook(() => useHarness(INITIAL));
    let ok = true;
    await act(async () => {
      ok = await result.current.optimistic.run({
        match: (it) => it.id === 'zzz',
        apply: (it) => it,
        commit,
      });
    });
    expect(ok).toBe(false);
    expect(commit).not.toHaveBeenCalled();
  });
});
