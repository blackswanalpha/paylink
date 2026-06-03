/**
 * mapAuthError — converts a server-side SDK throw into an HTTP status + LinkMint error envelope so the
 * browser can reconstruct the same typed error (the MFA_REQUIRED code must survive the route-handler hop).
 */

import { describe, it, expect } from 'vitest';
import { createApiError } from '@linkmint/sdk';

import { mapAuthError } from '@/lib/authSession';

describe('mapAuthError', () => {
  it('passes an SDK API error through as status + envelope (preserving code + trace)', () => {
    const err = createApiError({
      status: 401,
      code: 'MFA_REQUIRED',
      message: 'mfa required',
      details: { foo: 'bar' },
      traceId: 'trace-123',
      requestId: undefined,
    });

    const { status, body } = mapAuthError(err);

    expect(status).toBe(401);
    expect(body.error.code).toBe('MFA_REQUIRED');
    expect(body.error.message).toBe('mfa required');
    expect(body.error.trace_id).toBe('trace-123');
  });

  it('maps a non-SDK (transport) failure to 502 BAD_GATEWAY', () => {
    const { status, body } = mapAuthError(new Error('connection refused'));
    expect(status).toBe(502);
    expect(body.error.code).toBe('BAD_GATEWAY');
  });
});
