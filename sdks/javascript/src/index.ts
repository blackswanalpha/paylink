/**
 * `@linkmint/sdk` — typed TypeScript client for the LinkMint `/v1` API (PayLinks + payments).
 *
 * See {@link LinkMintClient}. The client talks only to the LinkMint API gateway and is
 * rail-agnostic (invariant A.4): it exposes no rail-specific PayLink fields.
 */

// Client
export { LinkMintClient, createClient } from './client';
export type { LinkMintClientOptions } from './client';

// Transport-level config + per-request options
export type { AuthConfig, FetchLike, RequestOptions } from './http';

// Resources (exported for typing / advanced composition)
export { PayLinksResource } from './resources/paylinks';
export { PaymentsResource } from './resources/payments';
export { NotificationsResource } from './resources/notifications';

// Idempotency helper
export { defaultIdempotencyKey } from './idempotency';

// Errors
export {
  LinkMintError,
  LinkMintConnectionError,
  LinkMintTimeoutError,
  LinkMintApiError,
  BadRequestError,
  UnauthorizedError,
  PaymentRequiredError,
  ForbiddenError,
  NotFoundError,
  ConflictError,
  RateLimitError,
  ServerError,
  createApiError,
  isLinkMintApiError,
  isErrorEnvelope,
} from './errors';
export type { ErrorCode, KnownErrorCode, ErrorEnvelope, ApiErrorInit } from './errors';

// Wire types
export type {
  PayLink,
  PayLinkStatus,
  PayLinkUsage,
  PaymentStatus,
  PaymentRail,
  CreatePayLinkInput,
  CreatePayLinkResult,
  CancelPayLinkResult,
  ListPayLinksParams,
  PayLinkList,
  InitiatePaymentInput,
  Payment,
  NotificationKind,
  Notification,
  ListNotificationsParams,
  NotificationList,
  MarkAllReadResult,
} from './types';
