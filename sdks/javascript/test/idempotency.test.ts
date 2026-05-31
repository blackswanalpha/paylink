import { describe, it, expect, vi, afterEach } from 'vitest';

import { defaultIdempotencyKey } from '../src/idempotency';

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

describe('defaultIdempotencyKey', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('returns a v4 UUID using Web Crypto when available', () => {
    expect(defaultIdempotencyKey()).toMatch(UUID_RE);
  });

  it('returns unique values', () => {
    expect(defaultIdempotencyKey()).not.toBe(defaultIdempotencyKey());
  });

  it('falls back to a generated v4 UUID when crypto is missing', () => {
    vi.stubGlobal('crypto', undefined);
    expect(defaultIdempotencyKey()).toMatch(UUID_RE);
  });

  it('falls back when crypto exists but lacks randomUUID', () => {
    vi.stubGlobal('crypto', {});
    expect(defaultIdempotencyKey()).toMatch(UUID_RE);
  });
});
