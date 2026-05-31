// Bootstrap: load config, build the Daraja client, start the HTTP server. Graceful shutdown on
// SIGTERM/SIGINT.

import { loadConfig } from './config.js';
import { DarajaClient } from './daraja.js';
import { StubDaraja } from './stub.js';
import { startServer } from './server.js';

const config = loadConfig();

function logLine(level, msg) {
  process.stdout.write(JSON.stringify({ service: 'mpesa-daraja-service', level, msg }) + '\n');
}

let daraja;
if (config.stub) {
  logLine('warn', 'DARAJA_STUB=true — using a stub Daraja client (no live STK pushes); devnet/e2e only');
  daraja = new StubDaraja();
} else {
  if (!config.consumerKey || !config.consumerSecret || !config.passkey) {
    logLine('warn', 'daraja credentials not fully set — /stk will fail until DARAJA_CONSUMER_KEY/SECRET/PASSKEY are provided');
  }
  daraja = new DarajaClient(config);
}
const server = startServer({ config, daraja });

function shutdown(sig) {
  process.stdout.write(JSON.stringify({ service: 'mpesa-daraja-service', level: 'info', msg: 'shutdown', signal: sig }) + '\n');
  server.close(() => process.exit(0));
  // Force-exit if connections linger.
  setTimeout(() => process.exit(0), 5000).unref();
}

process.on('SIGTERM', () => shutdown('SIGTERM'));
process.on('SIGINT', () => shutdown('SIGINT'));
