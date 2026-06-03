import { describe, it, expect } from 'vitest';

import { LinkMintClient } from '../src/client';
import { NotFoundError } from '../src/errors';
import { createMockFetch } from './helpers/mockFetch';

const BASE = 'https://api.linkmint.test';

describe('merchants.onboard / get / fee-tier', () => {
  it('POSTs /v1/merchants/onboard with the body and an idempotency key', async () => {
    const mock = createMockFetch({ status: 201, body: { merchant_id: 'm-1', status: 'DRAFT' } });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    const result = await client.merchants.onboard({
      org_id: 'o-1',
      business_name: 'Acme Ltd',
      country: 'KE',
      type: 'company',
    });

    expect(result).toEqual({ merchant_id: 'm-1', status: 'DRAFT' });
    const call = mock.lastCall();
    expect(call.url).toBe(`${BASE}/v1/merchants/onboard`);
    expect(call.body).toEqual({
      org_id: 'o-1',
      business_name: 'Acme Ltd',
      country: 'KE',
      type: 'company',
    });
    expect(call.headers['Idempotency-Key']).toBeTypeOf('string');
    expect(call.headers['X-Creator-Addr']).toBeUndefined();
  });

  it('includes registration_no when provided', async () => {
    const mock = createMockFetch({ status: 201, body: { merchant_id: 'm-2', status: 'DRAFT' } });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    await client.merchants.onboard({
      org_id: 'o-1',
      business_name: 'Acme',
      country: 'KE',
      type: 'company',
      registration_no: 'C-123',
    });
    expect(mock.lastCall().body).toMatchObject({ registration_no: 'C-123' });
  });

  it('GETs /v1/merchants/{id} and maps a 404 to NotFoundError', async () => {
    const ok = createMockFetch({ status: 200, body: { merchant_id: 'm-1', bank_accounts: [] } });
    const c1 = new LinkMintClient({ baseUrl: BASE, fetch: ok.fetch });
    await c1.merchants.get('m-1');
    expect(ok.lastCall().method).toBe('GET');
    expect(ok.lastCall().url).toBe(`${BASE}/v1/merchants/m-1`);

    const missing = createMockFetch({
      status: 404,
      body: { error: { code: 'MERCHANT_NOT_FOUND', message: 'no merchant' } },
    });
    const c2 = new LinkMintClient({ baseUrl: BASE, fetch: missing.fetch });
    await expect(c2.merchants.get('nope')).rejects.toBeInstanceOf(NotFoundError);
  });

  it('GETs and PATCHes the fee tier', async () => {
    const get = createMockFetch({
      status: 200,
      body: { merchant_id: 'm-1', tier: 'standard', effective_at: '2026-01-01T00:00:00Z' },
    });
    const c1 = new LinkMintClient({ baseUrl: BASE, fetch: get.fetch });
    const tier = await c1.merchants.feeTier('m-1');
    expect(tier.tier).toBe('standard');
    expect(get.lastCall().url).toBe(`${BASE}/v1/merchants/m-1/fee-tier`);
    expect(get.lastCall().headers['Idempotency-Key']).toBeUndefined();

    const patch = createMockFetch({
      status: 200,
      body: { merchant_id: 'm-1', tier: 'enterprise', effective_at: '2026-01-01T00:00:00Z' },
    });
    const c2 = new LinkMintClient({ baseUrl: BASE, fetch: patch.fetch });
    await c2.merchants.updateFeeTier('m-1', { tier: 'enterprise' });
    const call = patch.lastCall();
    expect(call.method).toBe('PATCH');
    expect(call.body).toEqual({ tier: 'enterprise' });
    expect(call.headers['Idempotency-Key']).toBeTypeOf('string');
  });
});

describe('merchants.addDocument (multipart)', () => {
  it('POSTs /v1/merchants/{id}/documents as FormData without a JSON content-type', async () => {
    const mock = createMockFetch({ status: 201, body: { document_id: 'd-1', status: 'UPLOADED' } });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });
    const file = new Blob(['hello'], { type: 'text/plain' });

    const result = await client.merchants.addDocument('m-1', { file, kind: 'cert_incorporation' });

    expect(result.status).toBe('UPLOADED');
    const call = mock.lastCall();
    expect(call.method).toBe('POST');
    expect(call.url).toBe(`${BASE}/v1/merchants/m-1/documents`);
    expect(call.body).toBeInstanceOf(FormData);
    expect(call.headers['Content-Type']).toBeUndefined();
    expect(call.headers['Idempotency-Key']).toBeTypeOf('string');

    const form = call.body as FormData;
    expect(form.get('kind')).toBe('cert_incorporation');
    expect(form.get('file')).toBeInstanceOf(Blob);
  });
});

describe('merchants bank accounts + contracts', () => {
  it('POSTs a bank account (secret in body, never a header)', async () => {
    const mock = createMockFetch({
      status: 201,
      body: { bank_account_id: 'b-1', status: 'PENDING' },
    });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    await client.merchants.addBankAccount('m-1', {
      rail: 'mpesa',
      account_details: '254700000000',
      currency: 'KES',
      country: 'KE',
    });
    const call = mock.lastCall();
    expect(call.url).toBe(`${BASE}/v1/merchants/m-1/bank-accounts`);
    expect(call.body).toEqual({
      rail: 'mpesa',
      account_details: '254700000000',
      currency: 'KES',
      country: 'KE',
    });
    expect(call.headers['Idempotency-Key']).toBeTypeOf('string');
  });

  it('POSTs bank-account verify (empty body when no amounts)', async () => {
    const mock = createMockFetch({
      status: 200,
      body: { bank_account_id: 'b-1', status: 'VERIFIED' },
    });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    await client.merchants.verifyBankAccount('m-1', 'b-1');
    const call = mock.lastCall();
    expect(call.url).toBe(`${BASE}/v1/merchants/m-1/bank-accounts/b-1/verify`);
    expect(call.body).toEqual({});

    const withAmounts = createMockFetch({
      status: 200,
      body: { bank_account_id: 'b-1', status: 'VERIFIED' },
    });
    const c2 = new LinkMintClient({ baseUrl: BASE, fetch: withAmounts.fetch });
    await c2.merchants.verifyBankAccount('m-1', 'b-1', { micro_deposit_amounts: [12, 34] });
    expect(withAmounts.lastCall().body).toEqual({ micro_deposit_amounts: [12, 34] });
  });

  it('POSTs accept-contract and GETs the contract list', async () => {
    const accept = createMockFetch({
      status: 201,
      body: {
        id: 1,
        merchant_id: 'm-1',
        version: 'v1',
        accepted_by: 'u-1',
        accepted_at: 't',
        ip: null,
        user_agent: null,
      },
    });
    const c1 = new LinkMintClient({ baseUrl: BASE, fetch: accept.fetch });
    await c1.merchants.acceptContract('m-1', { contract_version: 'v1', accepted: true });
    const call = accept.lastCall();
    expect(call.url).toBe(`${BASE}/v1/merchants/m-1/contracts`);
    expect(call.body).toEqual({ contract_version: 'v1', accepted: true });

    const list = createMockFetch({ status: 200, body: { items: [] } });
    const c2 = new LinkMintClient({ baseUrl: BASE, fetch: list.fetch });
    const result = await c2.merchants.listContracts('m-1');
    expect(result.items).toEqual([]);
    expect(list.lastCall().method).toBe('GET');
    expect(list.lastCall().headers['Idempotency-Key']).toBeUndefined();
  });
});
