// HTTP server for the Daraja rail service. Two inbound surfaces:
//   POST /stk             (from the Go core)  — start an STK push           [X-Internal-Token]
//   POST /daraja/callback (from Safaricom)    — STK result → forward to core [?t=<callbackToken>]
// Plus GET /healthz and /readyz. Built on node:http + built-ins only (no framework).

import http from 'node:http';
import crypto from 'node:crypto';
import { parseCallback, forwardToCore } from './callback.js';

function log(level, msg, fields = {}) {
  process.stdout.write(JSON.stringify({ service: 'mpesa-daraja-service', level, msg, ...fields }) + '\n');
}

function sendJSON(res, status, obj) {
  const body = JSON.stringify(obj);
  res.writeHead(status, { 'Content-Type': 'application/json' });
  res.end(body);
}

function readJSON(req) {
  return new Promise((resolve, reject) => {
    let data = '';
    req.on('data', (c) => {
      data += c;
      if (data.length > 1_000_000) {
        reject(new Error('request body too large'));
        req.destroy();
      }
    });
    req.on('end', () => {
      try {
        resolve(data ? JSON.parse(data) : {});
      } catch (e) {
        reject(e);
      }
    });
    req.on('error', reject);
  });
}

function tokenEqual(a, b) {
  const ab = Buffer.from(a || '', 'utf8');
  const bb = Buffer.from(b || '', 'utf8');
  if (ab.length !== bb.length) return false;
  return crypto.timingSafeEqual(ab, bb);
}

// createHandler builds the (req,res) handler. Deps are injectable for tests: daraja.stkPush(...) and
// forward(fields) -> { status, body }.
export function createHandler({ config, daraja, forward }) {
  const doForward =
    forward ||
    ((fields) => forwardToCore({ coreURL: config.coreURL, internalToken: config.internalToken, fields }));

  return async (req, res) => {
    const url = new URL(req.url, 'http://localhost');
    const path = url.pathname;

    try {
      if (req.method === 'GET' && path === '/healthz') {
        return sendJSON(res, 200, { status: 'ok' });
      }
      if (req.method === 'GET' && path === '/readyz') {
        return sendJSON(res, 200, { status: 'ready' });
      }

      if (req.method === 'POST' && path === '/stk') {
        if (config.internalToken && !tokenEqual(req.headers['x-internal-token'], config.internalToken)) {
          return sendJSON(res, 401, { error: { code: 'UNAUTHORIZED', message: 'invalid internal token' } });
        }
        const body = await readJSON(req);
        if (!body.shortcode || !body.payer_phone || !body.amount) {
          return sendJSON(res, 400, { error: { code: 'INVALID_PAYLOAD', message: 'shortcode, payer_phone, amount required' } });
        }
        try {
          const result = await daraja.stkPush({
            shortcode: body.shortcode,
            payerPhone: body.payer_phone,
            amount: body.amount,
            accountRef: body.account_ref,
            paylinkId: body.paylink_id,
          });
          log('info', 'stk_push_initiated', { checkout_request_id: result.checkout_request_id, receiver: body.shortcode });
          return sendJSON(res, 200, result);
        } catch (e) {
          log('error', 'stk_push_failed', { err: String(e.message || e) });
          return sendJSON(res, 502, { error: { code: 'DARAJA_UNAVAILABLE', message: String(e.message || e) } });
        }
      }

      if (req.method === 'POST' && path === '/daraja/callback') {
        if (config.callbackToken && !tokenEqual(url.searchParams.get('t'), config.callbackToken)) {
          log('warn', 'callback_bad_token');
          return sendJSON(res, 401, { error: { code: 'UNAUTHORIZED', message: 'invalid callback token' } });
        }
        let fields;
        try {
          fields = parseCallback(await readJSON(req));
        } catch (e) {
          log('warn', 'callback_unparseable', { err: String(e.message || e) });
          return sendJSON(res, 400, { error: { code: 'INVALID_PAYLOAD', message: String(e.message || e) } });
        }
        let forwarded;
        try {
          forwarded = await doForward(fields);
        } catch (e) {
          // Transport failure to the core: ask Daraja to redeliver.
          log('error', 'forward_failed', { err: String(e.message || e), checkout_request_id: fields.checkout_request_id });
          return sendJSON(res, 502, { ResultCode: 1, ResultDesc: 'core unavailable' });
        }
        // The core asks for redelivery on 5xx (e.g. validator temporarily down); otherwise ack so
        // Daraja stops retrying.
        if (forwarded.status >= 500) {
          log('warn', 'core_retryable', { core_status: forwarded.status, checkout_request_id: fields.checkout_request_id });
          return sendJSON(res, 502, { ResultCode: 1, ResultDesc: 'core retryable' });
        }
        log('info', 'callback_forwarded', { core_status: forwarded.status, outcome: forwarded.body?.status, checkout_request_id: fields.checkout_request_id });
        return sendJSON(res, 200, { ResultCode: 0, ResultDesc: 'Accepted' });
      }

      sendJSON(res, 404, { error: { code: 'NOT_FOUND', message: 'no such route' } });
    } catch (e) {
      log('error', 'unhandled', { err: String(e.message || e) });
      sendJSON(res, 500, { error: { code: 'INTERNAL_ERROR', message: 'internal error' } });
    }
  };
}

// startServer creates and listens an http.Server with the given handler.
export function startServer({ config, daraja, forward }) {
  const server = http.createServer(createHandler({ config, daraja, forward }));
  server.listen(config.port, () => log('info', 'listening', { port: config.port, core: config.coreURL }));
  return server;
}
