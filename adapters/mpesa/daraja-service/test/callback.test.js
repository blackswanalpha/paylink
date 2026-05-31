import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { parseCallback, forwardToCore } from '../src/callback.js';

const here = dirname(fileURLToPath(import.meta.url));
const fixture = (name) => JSON.parse(readFileSync(join(here, 'fixtures', name), 'utf8'));

test('parseCallback maps a successful STK callback to rail-neutral fields', () => {
  const f = parseCallback(fixture('stk_callback_success.json'));
  assert.equal(f.result_code, 0);
  assert.equal(f.amount, 1500);
  assert.equal(f.mpesa_receipt_number, 'NLJ7RT61SV');
  assert.equal(f.phone_number, '254708374149');
  assert.equal(f.checkout_request_id, 'ws_CO_191220191020363925');
});

test('parseCallback handles a failed payment (no metadata)', () => {
  const f = parseCallback(fixture('stk_callback_failed.json'));
  assert.equal(f.result_code, 1032);
  assert.equal(f.amount, 0);
  assert.equal(f.mpesa_receipt_number, '');
});

test('parseCallback rejects a body without CheckoutRequestID', () => {
  assert.throws(() => parseCallback({ Body: { stkCallback: { ResultCode: 0 } } }));
});

test('forwardToCore posts rail-neutral fields with the internal token', async () => {
  let captured;
  const fetchFn = async (url, opts) => {
    captured = { url, opts };
    return { status: 200, json: async () => ({ status: 'broadcast' }) };
  };
  const r = await forwardToCore({
    coreURL: 'http://core:8082/',
    internalToken: 'sec',
    fields: { checkout_request_id: 'ws_CO_1', amount: 1500 },
    fetchFn,
  });
  assert.equal(r.status, 200);
  assert.equal(captured.url, 'http://core:8082/v1/callbacks/mpesa');
  assert.equal(captured.opts.headers['X-Internal-Token'], 'sec');
  assert.equal(JSON.parse(captured.opts.body).amount, 1500);
});
