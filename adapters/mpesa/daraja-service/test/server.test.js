import { test } from 'node:test';
import assert from 'node:assert/strict';
import http from 'node:http';
import { createHandler } from '../src/server.js';

function listen(handler) {
  return new Promise((resolve) => {
    const s = http.createServer(handler);
    s.listen(0, () => resolve(s));
  });
}

async function call(base, method, path, { body, headers } = {}) {
  const resp = await fetch(base + path, {
    method,
    headers: { 'Content-Type': 'application/json', ...(headers || {}) },
    body: body ? JSON.stringify(body) : undefined,
  });
  const json = await resp.json().catch(() => ({}));
  return { status: resp.status, json };
}

test('GET /healthz returns ok', async () => {
  const s = await listen(createHandler({ config: {}, daraja: { stkPush: async () => ({}) }, forward: async () => ({ status: 200, body: {} }) }));
  const base = `http://localhost:${s.address().port}`;
  const r = await call(base, 'GET', '/healthz');
  assert.equal(r.status, 200);
  assert.equal(r.json.status, 'ok');
  s.close();
});

test('POST /stk requires the internal token then initiates a push', async () => {
  const cfg = { internalToken: 'sec' };
  const s = await listen(
    createHandler({ config: cfg, daraja: { stkPush: async () => ({ checkout_request_id: 'ws_1' }) }, forward: async () => ({ status: 200, body: {} }) }),
  );
  const base = `http://localhost:${s.address().port}`;

  let r = await call(base, 'POST', '/stk', { body: { shortcode: '600111', payer_phone: '254700000000', amount: 1500 } });
  assert.equal(r.status, 401, 'missing token rejected');

  r = await call(base, 'POST', '/stk', {
    body: { shortcode: '600111', payer_phone: '254700000000', amount: 1500 },
    headers: { 'X-Internal-Token': 'sec' },
  });
  assert.equal(r.status, 200);
  assert.equal(r.json.checkout_request_id, 'ws_1');
  s.close();
});

test('POST /daraja/callback validates the token, parses, forwards, and acks', async () => {
  let forwarded = null;
  const cfg = { callbackToken: 'tok' };
  const s = await listen(
    createHandler({
      config: cfg,
      daraja: { stkPush: async () => ({}) },
      forward: async (fields) => {
        forwarded = fields;
        return { status: 200, body: { status: 'broadcast' } };
      },
    }),
  );
  const base = `http://localhost:${s.address().port}`;
  const cb = {
    Body: {
      stkCallback: {
        CheckoutRequestID: 'ws_CO_1',
        ResultCode: 0,
        CallbackMetadata: {
          Item: [
            { Name: 'Amount', Value: 1500 },
            { Name: 'MpesaReceiptNumber', Value: 'R1' },
            { Name: 'PhoneNumber', Value: 254700000000 },
          ],
        },
      },
    },
  };

  let r = await call(base, 'POST', '/daraja/callback', { body: cb }); // no ?t
  assert.equal(r.status, 401, 'missing callback token rejected');

  r = await call(base, 'POST', '/daraja/callback?t=tok', { body: cb });
  assert.equal(r.status, 200);
  assert.equal(r.json.ResultCode, 0, 'Daraja ack');
  assert.equal(forwarded.checkout_request_id, 'ws_CO_1');
  assert.equal(forwarded.amount, 1500);
  assert.equal(forwarded.mpesa_receipt_number, 'R1');
  s.close();
});

test('POST /daraja/callback asks for redelivery when the core is unavailable (5xx)', async () => {
  const s = await listen(
    createHandler({
      config: { callbackToken: 'tok' },
      daraja: { stkPush: async () => ({}) },
      forward: async () => ({ status: 502, body: {} }),
    }),
  );
  const base = `http://localhost:${s.address().port}`;
  const cb = { Body: { stkCallback: { CheckoutRequestID: 'ws_CO_2', ResultCode: 0 } } };
  const r = await call(base, 'POST', '/daraja/callback?t=tok', { body: cb });
  assert.equal(r.status, 502, 'non-2xx so Daraja redelivers');
  s.close();
});
