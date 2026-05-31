// 12-factor config (env only). The Daraja credentials live HERE (the rail boundary), never in the
// Go core. Devnet/sandbox defaults make a local run work against the docker-compose network.

function env(key, def = '') {
  const v = process.env[key];
  return v === undefined || v === '' ? def : v;
}

function envBool(key, def) {
  const v = process.env[key];
  if (v === undefined || v === '') return def;
  return v === '1' || v.toLowerCase() === 'true';
}

export function loadConfig() {
  return {
    // HTTP listen address (port only).
    port: Number(env('DARAJA_SERVICE_PORT', '8083')),

    // Daraja (Safaricom) API.
    darajaBaseURL: env('DARAJA_BASE_URL', 'https://sandbox.safaricom.co.ke'),
    consumerKey: env('DARAJA_CONSUMER_KEY'),
    consumerSecret: env('DARAJA_CONSUMER_SECRET'),
    passkey: env('DARAJA_PASSKEY'),
    // Sandbox guard: refuse to talk to a non-sandbox host unless explicitly disabled.
    sandbox: envBool('DARAJA_SANDBOX', true),
    // Devnet/e2e only: stub the Daraja client (synthetic STK responses, no live calls). Never prod.
    stub: envBool('DARAJA_STUB', false),

    // The public URL Daraja POSTs results to (this service's /daraja/callback, token-protected).
    callbackURL: env('DARAJA_CALLBACK_URL', 'http://localhost:8083/daraja/callback'),
    // Shared secret Daraja must present on the callback URL (?t=...).
    callbackToken: env('DARAJA_CALLBACK_TOKEN'),

    // The Go adapter core we forward rail-neutral callbacks to.
    coreURL: env('ADAPTER_CORE_URL', 'http://localhost:8082'),
    // Shared secret authenticating the internal core↔rail hops.
    internalToken: env('MPESA_ADAPTER_INTERNAL_TOKEN', env('INTERNAL_TOKEN')),

    logLevel: env('DARAJA_LOG_LEVEL', 'info'),
  };
}
