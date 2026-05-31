/**
 * Idempotency-key generation.
 *
 * The SDK attaches an `Idempotency-Key` to every state-mutating request: `POST /v1/payments`
 * *requires* one (payment-orchestrator `payments.go`), and paylink-service honors it when present,
 * so generating one by default makes retries safe everywhere. Callers can override per request.
 */

interface RandomUuidCrypto {
  randomUUID(): string;
}

function hasRandomUuid(value: unknown): value is RandomUuidCrypto {
  return (
    typeof value === 'object' &&
    value !== null &&
    typeof (value as { randomUUID?: unknown }).randomUUID === 'function'
  );
}

/**
 * Generate a fresh idempotency key (a UUID v4). Uses the Web Crypto `crypto.randomUUID()` when
 * available (Node 16+, modern browsers); otherwise falls back to a non-cryptographic v4 generator,
 * which is sufficient for uniqueness of an idempotency token.
 */
export function defaultIdempotencyKey(): string {
  const cryptoObj: unknown = globalThis.crypto;
  if (hasRandomUuid(cryptoObj)) {
    return cryptoObj.randomUUID();
  }
  return fallbackUuidV4();
}

function fallbackUuidV4(): string {
  // RFC 4122 v4 layout using Math.random — only reached when Web Crypto is unavailable.
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (char) => {
    const rand = (Math.random() * 16) | 0;
    const value = char === 'x' ? rand : (rand & 0x3) | 0x8;
    return value.toString(16);
  });
}
