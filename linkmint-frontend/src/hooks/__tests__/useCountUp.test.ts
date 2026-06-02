import { describe, it, expect } from 'vitest';
import { renderHook } from '@testing-library/react';
import { useCountUp } from '../useCountUp';

describe('useCountUp', () => {
  it('settles on the real target value, never a fake number (F.7)', () => {
    const { result } = renderHook(() => useCountUp(2500));
    expect(result.current).toBe(2500);
  });

  it('returns an integer (money is integer minor units)', () => {
    const { result } = renderHook(() => useCountUp(1999));
    expect(Number.isInteger(result.current)).toBe(true);
  });

  it('jumps straight to the target under reduced motion (F.6)', () => {
    const original = window.matchMedia;
    window.matchMedia = ((query: string) => ({
      matches: true,
      media: query,
      onchange: null,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
      addListener: () => undefined,
      removeListener: () => undefined,
      dispatchEvent: () => false,
    })) as unknown as typeof window.matchMedia;
    try {
      const { result } = renderHook(() => useCountUp(500));
      expect(result.current).toBe(500);
    } finally {
      window.matchMedia = original;
    }
  });
});
