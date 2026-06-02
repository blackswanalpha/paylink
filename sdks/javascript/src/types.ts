/**
 * Wire types for the LinkMint `/v1` API.
 *
 * These mirror the server response/request JSON **exactly** (snake_case field names) so the
 * SDK is a faithful, transformation-free contract for clients. They are sourced from:
 *   - paylink-service  `app/api/v1/schemas.py`, `app/domain/models.py`
 *   - payment-orchestrator `internal/server/payments.go`, `internal/lifecycle/lifecycle.go`
 *
 * Invariant A.4 (rail-agnostic): a {@link PayLink} carries **no** rail-specific fields. The only
 * rail reference in the whole surface is {@link PaymentRail} — an opaque routing label chosen when
 * initiating a payment. `metadata`/`rules` are opaque JSON and must not carry fund-moving data.
 */

/** Lifecycle status of a PayLink (paylink-service `app/domain/models.py`). */
export type PayLinkStatus = 'CREATED' | 'PENDING' | 'VERIFIED' | 'FAILED' | 'CANCELLED' | 'EXPIRED';

/** How many times a PayLink may be paid. */
export type PayLinkUsage = 'single' | 'multi';

/** Lifecycle status of a payment (payment-orchestrator `internal/lifecycle/lifecycle.go`). */
export type PaymentStatus = 'AWAITING_PAYMENT' | 'SETTLED' | 'CANCELLED' | 'FAILED';

/** Opaque payment-rail routing label. Not a rail-specific field on the PayLink (invariant A.4). */
export type PaymentRail = 'mpesa' | 'card' | 'bank' | 'crypto';

/** A PayLink as returned by `GET /v1/paylinks/{pl_id}` and within list results. */
export interface PayLink {
  /** 0x-prefixed 32-byte (64 hex char) PayLink id. */
  pl_id: string;
  /** 0x-prefixed 20-byte address that created the link. */
  creator: string;
  /** 0x-prefixed 20-byte address that receives the funds. */
  receiver: string;
  /** 0x-prefixed 20-byte current owner (may differ from creator). */
  owner: string;
  /** Amount in integer minor units (e.g. cents). */
  amount: number;
  /** ISO 4217 code or PLN. */
  currency: string;
  status: PayLinkStatus;
  /** RFC 3339 / ISO 8601 timestamp. */
  expiry: string;
  usage: PayLinkUsage;
  /** Number of validator votes received on-chain. */
  vote_count: number;
  /** On-chain creation tx hash, or null if not yet submitted. */
  chain_tx_hash: string | null;
  created_at: string;
  updated_at: string;
  /** Set when the PayLink settled (VERIFIED), otherwise null. */
  verified_at: string | null;
}

/** Request body for `POST /v1/paylinks`. */
export interface CreatePayLinkInput {
  /** 0x-prefixed 20-byte hex address that will receive the funds. */
  receiver: string;
  /** Positive amount in integer minor units. */
  amount: number;
  /** Expiry instant. Accepts an ISO 8601 string or a `Date` (serialized to ISO). Must be future. */
  expiry: string | Date;
  /** ISO 4217 or PLN. Defaults to the server's configured currency when omitted. */
  currency?: string;
  /** Defaults to `"single"` server-side when omitted. */
  usage?: PayLinkUsage;
  /** Opaque JSON metadata. Must not carry fund-moving / rail-specific data (invariants A.1/A.4). */
  metadata?: Record<string, unknown>;
  /** Opaque JSON rules blob, passed through untouched by the service. */
  rules?: unknown;
}

/** Response of `POST /v1/paylinks` (201). */
export interface CreatePayLinkResult {
  pl_id: string;
  status: PayLinkStatus;
  created_at: string;
  chain_tx_hash: string | null;
}

/** Response of `POST /v1/paylinks/{pl_id}/cancel` (200). */
export interface CancelPayLinkResult {
  pl_id: string;
  status: PayLinkStatus;
}

/** Query parameters for `GET /v1/paylinks`. */
export interface ListPayLinksParams {
  /** Filter by creator address (case-insensitive). */
  creator?: string;
  /** Filter by receiver address (case-insensitive). */
  receiver?: string;
  /** Filter by status (case-insensitive). */
  status?: PayLinkStatus;
  /** Page size, 1–100. Defaults to 20 server-side. */
  limit?: number;
  /** Opaque pagination cursor from a previous page's `next_cursor`. */
  cursor?: string;
}

/** Response of `GET /v1/paylinks` (200). */
export interface PayLinkList {
  items: PayLink[];
  /** Cursor for the next page, or null when there are no more results. */
  next_cursor: string | null;
}

/** Request body for `POST /v1/payments`. */
export interface InitiatePaymentInput {
  /** 0x-prefixed 32-byte PayLink id to pay. */
  paylink_id: string;
  /** Opaque rail routing label. */
  rail: PaymentRail;
}

/** A payment as returned by `POST /v1/payments` (201) and `GET /v1/payments/{id}` (200). */
export interface Payment {
  /** UUID payment id. */
  id: string;
  paylink_id: string;
  rail: string;
  status: PaymentStatus;
  created_at: string;
  updated_at: string;
}

/** Severity of an in-app notification (drives the notification center's colour-coding). */
export type NotificationKind = 'success' | 'info' | 'warning' | 'error';

/**
 * An in-app notification as returned by the notification center API
 * (notification-service `app/api/v1/inbox.py`). Scoped server-side to the authenticated caller, so
 * the surface carries no recipient field.
 */
export interface Notification {
  /** UUID notification id. */
  id: string;
  kind: NotificationKind;
  title: string;
  /** Body / detail line, or null. */
  body: string | null;
  /** Optional in-app deep link to open when activated, or null. */
  href: string | null;
  read: boolean;
  /** RFC 3339 / ISO 8601 timestamp. */
  created_at: string;
}

/** Query parameters for `GET /v1/notifications`. */
export interface ListNotificationsParams {
  /** Page size, 1–100. Defaults to 20 server-side. */
  limit?: number;
  /** Opaque pagination cursor from a previous page's `next_cursor`. */
  cursor?: string;
}

/** Response of `GET /v1/notifications` (200). */
export interface NotificationList {
  items: Notification[];
  /** Cursor for the next page, or null when there are no more results. */
  next_cursor: string | null;
}

/** Response of `POST /v1/notifications/read-all` (200). */
export interface MarkAllReadResult {
  /** How many notifications were flipped to read. */
  count: number;
}
