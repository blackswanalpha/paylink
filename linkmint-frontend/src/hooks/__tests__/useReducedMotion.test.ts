import { describe, it, expect } from 'vitest';
import { renderHook } from '@testing-library/react';
import { useReducedMotion } from '../useReducedMotion';

describe('useReducedMotion', () => {
  it('is falsy when the OS does not request reduced motion (default matchMedia stub)', () => {
    const { result } = renderHook(() => useReducedMotion());
    // The setup stub reports matches:false, so motion is enabled. (Tolerate null/false across versions.)
    expect(result.current).not.toBe(true);
  });
});
