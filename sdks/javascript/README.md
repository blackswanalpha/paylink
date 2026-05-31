# @linkmint/sdk

Typed TypeScript/JavaScript client for the LinkMint **`/v1`** API — PayLinks and payments.
It is the supported way for the web app and partners to call LinkMint; **prefer it over raw
`fetch`** so API usage stays consistent and type-safe.

- Strict TypeScript, **no `any`**. Ships ESM + CommonJS + `.d.ts`.
- **Zero runtime dependencies** — uses the platform `fetch` (Node 18+, modern browsers, edge runtimes).
- Bearer-JWT / API-key auth passed straight through to the API gateway.
- The standard LinkMint error envelope mapped to **typed errors**.
- Rail-agnostic (protocol invariant A.4): exposes **no** rail-specific PayLink fields.

## Install

```bash
npm install @linkmint/sdk
```

## Quickstart

```ts
import { LinkMintClient } from '@linkmint/sdk';

const linkmint = new LinkMintClient({
  baseUrl: process.env.LINKMINT_API_URL!, // the API gateway, e.g. https://api.linkmint.example
  auth: { type: 'bearer', token: process.env.LINKMINT_JWT! },
});

// Create a PayLink
const link = await linkmint.paylinks.create({
  receiver: '0x1111111111111111111111111111111111111111',
  amount: 1000, // integer minor units
  expiry: new Date(Date.now() + 24 * 60 * 60 * 1000), // Date or ISO string
  currency: 'USD',
});

// Initiate a payment against it (rail is an opaque routing label)
const payment = await linkmint.payments.initiate({ paylink_id: link.pl_id, rail: 'mpesa' });

// Poll its status (reconciled against on-chain truth server-side)
const status = await linkmint.payments.get(payment.id);
console.log(status.status); // 'AWAITING_PAYMENT' | 'SETTLED' | 'CANCELLED' | 'FAILED'
```

## Configuration

`LinkMintClient` is 12-factor — pass values from the environment:

| Option                   | Type                    | Default        | Notes                                                     |
| ------------------------ | ----------------------- | -------------- | --------------------------------------------------------- |
| `baseUrl`                | `string` (required)     | —              | API gateway base URL.                                     |
| `auth`                   | `AuthConfig`            | none           | `{ type: 'bearer', token }` or `{ type: 'apiKey', key }`. |
| `fetch`                  | `typeof fetch`          | global `fetch` | Inject for tests / custom runtimes.                       |
| `timeoutMs`              | `number`                | `30000`        | Per-request timeout.                                      |
| `defaultHeaders`         | `Record<string,string>` | `{}`           | Added to every request.                                   |
| `generateIdempotencyKey` | `() => string`          | UUID v4        | Override the idempotency-key generator.                   |

### Authentication

```ts
// JWT (string or async provider for refresh)
new LinkMintClient({ baseUrl, auth: { type: 'bearer', token: () => getFreshJwt() } });

// Partner API key (sent as X-API-Key)
new LinkMintClient({ baseUrl, auth: { type: 'apiKey', key: process.env.LINKMINT_API_KEY! } });
```

The gateway derives the caller's chain address from the verified credential and injects it
downstream — the SDK never sends `X-Creator-Addr`.

## Resources

### `paylinks`

| Method                    | Endpoint                           | Returns               |
| ------------------------- | ---------------------------------- | --------------------- |
| `create(input, options?)` | `POST /v1/paylinks`                | `CreatePayLinkResult` |
| `get(plId, options?)`     | `GET /v1/paylinks/{pl_id}`         | `PayLink`             |
| `list(params?, options?)` | `GET /v1/paylinks`                 | `PayLinkList`         |
| `cancel(plId, options?)`  | `POST /v1/paylinks/{pl_id}/cancel` | `CancelPayLinkResult` |

### `payments`

| Method                      | Endpoint                | Returns   |
| --------------------------- | ----------------------- | --------- |
| `initiate(input, options?)` | `POST /v1/payments`     | `Payment` |
| `get(id, options?)`         | `GET /v1/payments/{id}` | `Payment` |

### Pagination

```ts
let cursor: string | undefined;
do {
  const page = await linkmint.paylinks.list({ creator, limit: 50, cursor });
  for (const link of page.items) handle(link);
  cursor = page.next_cursor ?? undefined;
} while (cursor);
```

### Idempotency

Mutating calls (`paylinks.create`, `paylinks.cancel`, `payments.initiate`) automatically send an
`Idempotency-Key` (a UUID) so retries are safe. Supply your own to make a retry idempotent across
process restarts:

```ts
await linkmint.payments.initiate({ paylink_id, rail: 'card' }, { idempotencyKey: myStableKey });
```

## Error handling

Every `>= 400` response is thrown as a typed error carrying the envelope fields:

```ts
import { LinkMintApiError, NotFoundError, ConflictError, RateLimitError } from '@linkmint/sdk';

try {
  await linkmint.paylinks.cancel(plId);
} catch (err) {
  if (err instanceof ConflictError && err.code === 'PAYLINK_ALREADY_SETTLED') {
    // already settled — nothing to cancel
  } else if (err instanceof NotFoundError) {
    // no such PayLink
  } else if (err instanceof RateLimitError) {
    await sleep((err.retryAfter ?? 1) * 1000);
  } else if (err instanceof LinkMintApiError) {
    console.error(err.code, err.status, err.message, err.traceId);
  } else {
    throw err; // LinkMintConnectionError / LinkMintTimeoutError, or a programmer error
  }
}
```

Error classes: `LinkMintError` (base) → `LinkMintApiError` (`status`, `code`, `details`,
`traceId`, `requestId`) with status-mapped subclasses `BadRequestError` (400),
`UnauthorizedError` (401), `PaymentRequiredError` (402), `ForbiddenError` (403),
`NotFoundError` (404), `ConflictError` (409), `RateLimitError` (429, `retryAfter`),
`ServerError` (5xx); and `LinkMintConnectionError` → `LinkMintTimeoutError` for transport failures.
`err.code` is typed against the known LinkMint error codes while still accepting future codes.

## Development

```bash
npm install
npm run typecheck   # tsc --noEmit (strict)
npm test            # vitest
npm run test:cov    # vitest + coverage (>= 80%)
npm run lint        # eslint + prettier --check
npm run build       # tsup -> dist (esm + cjs + d.ts)
```

## Versioning

The SDK is updated in lockstep with the `/v1` endpoints it consumes. It is **rail-agnostic**:
only `PaymentRail` (`'mpesa' | 'card' | 'bank' | 'crypto'`) appears, as the payment routing label —
PayLinks themselves carry no rail-specific fields.
