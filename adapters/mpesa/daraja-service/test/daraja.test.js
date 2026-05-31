import { test } from 'node:test';
import assert from 'node:assert/strict';
import { DarajaClient, darajaTimestamp, stkPassword } from '../src/daraja.js';

const baseCfg = {
  darajaBaseURL: 'https://sandbox.safaricom.co.ke',
  consumerKey: 'ck',
  consumerSecret: 'cs',
  passkey: 'pk',
  sandbox: true,
  callbackURL: 'http://mpesa-daraja:8083/daraja/callback?t=tok',
};

const fixedNow = () => new Date('2024-01-02T03:04:05Z');

test('darajaTimestamp + stkPassword', () => {
  assert.equal(darajaTimestamp(new Date('2024-01-02T03:04:05Z')), '20240102030405');
  assert.equal(
    stkPassword('174379', 'pk', '20240102030405'),
    Buffer.from('174379pk20240102030405', 'utf8').toString('base64'),
  );
});

test('sandbox guard rejects a non-sandbox host', () => {
  assert.throws(() => new DarajaClient({ ...baseCfg, darajaBaseURL: 'https://api.safaricom.co.ke' }, {}));
});

test('getToken caches until expiry', async () => {
  let calls = 0;
  const fetchFn = async () => {
    calls++;
    return { ok: true, json: async () => ({ access_token: 'tok', expires_in: '3599' }) };
  };
  const c = new DarajaClient(baseCfg, { fetchFn, now: fixedNow });
  assert.equal(await c.getToken(), 'tok');
  assert.equal(await c.getToken(), 'tok');
  assert.equal(calls, 1, 'token should be fetched once and cached');
});

test('stkPush sends the receiver as BusinessShortCode/PartyB with a Bearer token (A.1)', async () => {
  let captured;
  const fetchFn = async (url, opts) => {
    if (url.includes('/oauth/')) {
      return { ok: true, json: async () => ({ access_token: 'tok', expires_in: '3599' }) };
    }
    captured = { url, opts };
    return {
      ok: true,
      json: async () => ({ MerchantRequestID: 'm1', CheckoutRequestID: 'ws_CO_1', ResponseCode: '0', CustomerMessage: 'ok' }),
    };
  };
  const c = new DarajaClient(baseCfg, { fetchFn, now: fixedNow });

  const res = await c.stkPush({ shortcode: '600111', payerPhone: '254700000000', amount: 1500, accountRef: 'abc123', paylinkId: '0xabc' });
  assert.equal(res.checkout_request_id, 'ws_CO_1');

  assert.ok(captured.url.endsWith('/mpesa/stkpush/v1/processrequest'));
  assert.equal(captured.opts.headers.Authorization, 'Bearer tok');
  const body = JSON.parse(captured.opts.body);
  assert.equal(body.BusinessShortCode, '600111');
  assert.equal(body.PartyB, '600111', 'PartyB must be the receiver shortcode (A.1)');
  assert.equal(body.PartyA, '254700000000');
  assert.equal(body.PhoneNumber, '254700000000');
  assert.equal(body.Amount, '1500');
  assert.equal(body.AccountReference, 'abc123');
  assert.equal(body.CallBackURL, baseCfg.callbackURL);
});

test('stkPush surfaces a Daraja error', async () => {
  const fetchFn = async (url) => {
    if (url.includes('/oauth/')) return { ok: true, json: async () => ({ access_token: 't', expires_in: '3599' }) };
    return { ok: false, status: 500, json: async () => ({ errorMessage: 'boom' }) };
  };
  const c = new DarajaClient(baseCfg, { fetchFn, now: fixedNow });
  await assert.rejects(() => c.stkPush({ shortcode: '1', payerPhone: '254', amount: 1 }));
});
