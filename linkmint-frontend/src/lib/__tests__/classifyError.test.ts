/**
 * classifyError — the pure decision table at the heart of the work04 error system. One assertion per
 * row, with emphasis on the load-bearing rules: 401 is forced, 402 is overridable, 409 is never
 * retryable (mutation conflict), and 429 passes `retryAfter` through.
 */

import { describe, it, expect } from 'vitest';
import { classifyError, type DisplayError } from '@/lib/errors';

function api(status: number, extra: Partial<DisplayError> = {}): DisplayError {
  return { kind: 'api', title: 't', message: 'm', status, ...extra };
}

describe('classifyError', () => {
  it('400 → inline, not retryable', () => {
    expect(classifyError(api(400))).toMatchObject({
      surface: 'inline',
      canRetry: false,
      forced: false,
    });
  });

  it('401 → reauth, forced (cannot be downgraded)', () => {
    expect(classifyError(api(401))).toMatchObject({
      surface: 'reauth',
      forced: true,
      canRetry: false,
    });
  });

  it('402 KYC_REQUIRED → kyc, NOT forced (overridable to an inline gate)', () => {
    expect(classifyError(api(402, { code: 'KYC_REQUIRED' }))).toMatchObject({
      surface: 'kyc',
      forced: false,
    });
  });

  it('403 → inline', () => {
    expect(classifyError(api(403)).surface).toBe('inline');
  });

  it('404 → inline', () => {
    expect(classifyError(api(404)).surface).toBe('inline');
  });

  it('409 → inline and never retryable (mutation conflict)', () => {
    expect(classifyError(api(409, { code: 'PAYLINK_ALREADY_SETTLED' }))).toMatchObject({
      surface: 'inline',
      canRetry: false,
    });
  });

  it('429 → inline, retryable, with retryAfter passed through', () => {
    expect(classifyError(api(429, { retryAfter: 12 }))).toMatchObject({
      surface: 'inline',
      canRetry: true,
      retryAfter: 12,
    });
  });

  it('5xx → toast, retryable', () => {
    expect(classifyError(api(503))).toMatchObject({ surface: 'toast', canRetry: true });
  });

  it('transport → toast, retryable', () => {
    expect(classifyError({ kind: 'transport', title: 't', message: 'm' })).toMatchObject({
      surface: 'toast',
      canRetry: true,
    });
  });

  it('unknown → toast, not retryable', () => {
    expect(classifyError({ kind: 'unknown', title: 't', message: 'm' })).toMatchObject({
      surface: 'toast',
      canRetry: false,
    });
  });
});
