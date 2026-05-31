import { describe, it, expect } from 'vitest';

import { LinkMintClient } from '../src/client';
import { ConflictError, NotFoundError } from '../src/errors';
import type { Payment } from '../src/types';
import { createMockFetch } from './helpers/mockFetch';

const BASE = 'https://api.linkmint.test';

function samplePayment(overrides: Partial<Payment> = {}): Payment {
  return {
    id: '11111111-1111-1111-1111-111111111111',
    paylink_id: '0x' + 'a'.repeat(64),
    rail: 'mpesa',
    status: 'AWAITING_PAYMENT',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

describe('payments.initiate', () => {
  it('POSTs /v1/payments with paylink_id + rail and an auto idempotency key (required by orchestrator)', async () => {
    const mock = createMockFetch({ status: 201, body: samplePayment() });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    const payment = await client.payments.initiate({
      paylink_id: '0x' + 'a'.repeat(64),
      rail: 'mpesa',
    });

    expect(payment.status).toBe('AWAITING_PAYMENT');
    expect(payment.rail).toBe('mpesa');

    const call = mock.lastCall();
    expect(call.method).toBe('POST');
    expect(call.url).toBe(`${BASE}/v1/payments`);
    expect(call.body).toEqual({ paylink_id: '0x' + 'a'.repeat(64), rail: 'mpesa' });
    // payment-orchestrator requires Idempotency-Key — the SDK always supplies one.
    expect(call.headers['Idempotency-Key']).toBeTypeOf('string');
    expect(call.headers['Idempotency-Key']?.length).toBeGreaterThan(0);
  });

  it('maps 409 PAYLINK_NOT_PAYABLE to ConflictError', async () => {
    const mock = createMockFetch({
      status: 409,
      body: { error: { code: 'PAYLINK_NOT_PAYABLE', message: 'not payable' } },
    });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    await expect(
      client.payments.initiate({ paylink_id: '0xabc', rail: 'card' }),
    ).rejects.toMatchObject({ name: 'ConflictError', code: 'PAYLINK_NOT_PAYABLE', status: 409 });
  });

  it('maps 409 PAYMENT_EXISTS to ConflictError', async () => {
    const mock = createMockFetch({
      status: 409,
      body: { error: { code: 'PAYMENT_EXISTS', message: 'already initiated' } },
    });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    await expect(
      client.payments.initiate({ paylink_id: '0xabc', rail: 'crypto' }),
    ).rejects.toBeInstanceOf(ConflictError);
  });
});

describe('payments.get', () => {
  it('GETs /v1/payments/{id} and returns the reconciled status', async () => {
    const payment = samplePayment({ status: 'SETTLED', updated_at: '2026-01-02T00:00:00Z' });
    const mock = createMockFetch({ status: 200, body: payment });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    const result = await client.payments.get(payment.id);

    expect(result).toEqual(payment);
    const call = mock.lastCall();
    expect(call.method).toBe('GET');
    expect(call.url).toBe(`${BASE}/v1/payments/${payment.id}`);
  });

  it('maps 404 PAYMENT_NOT_FOUND to NotFoundError', async () => {
    const mock = createMockFetch({
      status: 404,
      body: { error: { code: 'PAYMENT_NOT_FOUND', message: 'not found' } },
    });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    await expect(client.payments.get('missing')).rejects.toBeInstanceOf(NotFoundError);
  });
});
