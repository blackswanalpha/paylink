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

/** A toggleable delivery channel: the in-app inbox, email, or SMS. */
export type NotificationChannel = 'in_app' | 'email' | 'sms';

/** An event kind a recipient can opt out of (mirrors notification-service `KNOWN_EVENTS`). */
export type NotificationEventKind =
  | 'paylink.created'
  | 'paylink.verified'
  | 'paylink.cancelled'
  | 'payment.failed';

/**
 * A recipient's notification preferences (notification-service `app/api/v1/preferences.py`).
 * Opt-out model: a channel/event is on unless turned off. `GET` returns the full effective set.
 * Scoped server-side to the caller's creator address (the same seam as the inbox), so there is no
 * recipient field.
 */
export interface NotificationPreferences {
  /** Per-channel master switches. */
  channels: Record<NotificationChannel, boolean>;
  /** Per-event-kind switches. */
  events: Record<NotificationEventKind, boolean>;
  /** ISO 8601 timestamp of the last save, or null if never saved. */
  updated_at: string | null;
}

/** Body for `PUT /v1/notifications/preferences` — a patch; only the keys present change. */
export interface UpdateNotificationPreferencesInput {
  channels?: Partial<Record<NotificationChannel, boolean>>;
  events?: Partial<Record<NotificationEventKind, boolean>>;
}

/*
 * ─────────────────────────────────────────────────────────────────────────────────────────────
 * identity-service — auth / users / organizations / sessions (work09).
 * Sourced from `identity-service/app/api/v1/schemas.py` + `app/domain/models.py`. The frontend
 * never sends `X-Creator-Addr`; these endpoints authenticate via the RS256 bearer token the gateway
 * forwards (`auth.login` mints it).
 * ─────────────────────────────────────────────────────────────────────────────────────────────
 */

/** Organization category (`OrgType` in identity `app/domain/models.py`). */
export type OrgType = 'merchant' | 'developer' | 'admin';

/** Org-scoped membership role (`Role`). The RBAC source of truth for downstream services. */
export type Role = 'owner' | 'admin' | 'developer' | 'operator' | 'viewer';

/** API-key scope (`Scope`) — the gateway-facing subset granted per role. */
export type Scope = 'paylinks:read' | 'paylinks:write' | 'payments:read' | 'payments:write';

/** Request body for `POST /v1/auth/register`. One of `email`/`phone` is required. */
export interface RegisterInput {
  email?: string;
  phone?: string;
  /** 8–256 chars. */
  password: string;
}

/** Response of `POST /v1/auth/register` (201). */
export interface RegisterResult {
  user_id: string;
  status: string;
}

/** Request body for `POST /v1/auth/login`. Provide exactly one of `email`/`phone`. */
export interface LoginInput {
  email?: string;
  phone?: string;
  password: string;
  /** TOTP code, required only when the account has MFA enabled. */
  mfa_code?: string;
}

/** OAuth2-style token pair returned by login / refresh / oauth callback. */
export interface TokenResponse {
  access_token: string;
  refresh_token: string;
  /** Always `"Bearer"`. */
  token_type: string;
  /** Access-token lifetime in seconds. */
  expires_in: number;
}

/** Request body for `POST /v1/auth/refresh`. */
export interface RefreshInput {
  refresh_token: string;
}

/** Request body for `POST /v1/auth/logout` (requires a bearer token). */
export interface LogoutInput {
  refresh_token: string;
}

/** Response of `POST /v1/auth/logout` (200). */
export interface LogoutResult {
  status: string;
}

/** Request body for `POST /v1/auth/oauth/{provider}/start`. */
export interface OAuthStartInput {
  redirect_uri?: string;
  state?: string;
}

/** Response of `POST /v1/auth/oauth/{provider}/start` (200). */
export interface OAuthStartResult {
  authorize_url: string;
  state: string;
}

/** Request body for `POST /v1/auth/oauth/{provider}/callback`. */
export interface OAuthCallbackInput {
  code: string;
  state?: string;
  redirect_uri?: string;
}

/** Response of `POST /v1/auth/mfa/enroll` (200) — the one-time TOTP secret (never cached). */
export interface MfaEnrollResult {
  secret: string;
  otpauth_uri: string;
}

/** Request body for `POST /v1/auth/mfa/{verify,disable}`. */
export interface MfaCodeInput {
  code: string;
}

/** Response of `POST /v1/auth/mfa/verify` (200). */
export interface MfaVerifyResult {
  enabled: boolean;
}

/** Response of `POST /v1/auth/mfa/disable` (200). */
export interface MfaDisableResult {
  status: string;
}

/** Request body for `POST /v1/auth/password/reset-request`. Provide exactly one of `email`/`phone`. */
export interface PasswordResetRequestInput {
  email?: string;
  phone?: string;
}

/**
 * Response of `POST /v1/auth/password/reset-request` (200). Always returns the same shape whether or
 * not the account exists (anti-enumeration). `reset_token` is populated only in dev; null otherwise.
 */
export interface PasswordResetRequestResult {
  status: string;
  reset_token?: string | null;
}

/** Request body for `POST /v1/auth/password/reset-confirm`. */
export interface PasswordResetConfirmInput {
  token: string;
  /** 8–256 chars. */
  new_password: string;
}

/** Response of `POST /v1/auth/password/reset-confirm` (200). */
export interface PasswordResetConfirmResult {
  status: string;
}

/** An (org_id, role) pair within a {@link UserProfile}. */
export interface OrgRoleEntry {
  org_id: string;
  role: string;
}

/** Response of `GET /v1/users/me` (200). No password/MFA/key material is ever included. */
export interface UserProfile {
  user_id: string;
  email: string | null;
  phone: string | null;
  kyc_tier: number;
  status: string;
  /** Whether the account has an active (verified) MFA factor. */
  mfa_enabled: boolean;
  roles: OrgRoleEntry[];
  user_roles: string[];
  created_at: string;
  last_login_at: string | null;
}

/** Request body for `PATCH /v1/users/me`. */
export interface UpdateProfileInput {
  email?: string;
  phone?: string;
}

/** Request body for `POST /v1/users/me/api-keys`. */
export interface IssueApiKeyInput {
  org_id: string;
  name: string;
  /** Defaults to `[]` server-side when omitted. */
  scopes?: Scope[];
}

/** Response of `POST /v1/users/me/api-keys` (201) — the only place `full_key` is ever returned. */
export interface IssueApiKeyResult {
  api_key_id: string;
  org_id: string;
  name: string;
  prefix: string;
  /** The full secret key, shown EXACTLY once at issuance; `null` on an idempotent replay. */
  full_key: string | null;
  scopes: string[];
  status: string;
  created_at: string;
}

/** An API key as returned by `GET /v1/users/me/api-keys` (no secret material). */
export interface ApiKey {
  api_key_id: string;
  org_id: string;
  name: string;
  prefix: string;
  scopes: string[];
  status: string;
  created_at: string;
  revoked_at: string | null;
}

/** Response of `GET /v1/users/me/api-keys` (200). */
export interface ApiKeyList {
  items: ApiKey[];
}

/** Response of `DELETE /v1/users/me/api-keys/{id}` (200). */
export interface RevokeApiKeyResult {
  api_key_id: string;
  status: string;
}

/** Request body for `POST /v1/organizations`. */
export interface CreateOrgInput {
  name: string;
  type: OrgType;
}

/** Response of `POST /v1/organizations` (201). `role` is the creator's membership role. */
export interface Org {
  org_id: string;
  name: string;
  type: string;
  role: string;
  created_at: string;
}

/** Response of `GET /v1/organizations` (200) — the caller's organizations, newest first. */
export interface OrgList {
  items: Org[];
}

/** Request body for `POST /v1/organizations/{orgId}/members`. Provide exactly one of `user_id`/`email`. */
export interface AddMemberInput {
  user_id?: string;
  email?: string;
  role: Role;
}

/** A membership row as returned by the organizations API. */
export interface Member {
  org_id: string;
  user_id: string;
  role: string;
}

/** Response of `GET /v1/organizations/{orgId}/members` (200). */
export interface MemberList {
  items: Member[];
}

/** Response of `DELETE /v1/organizations/{orgId}/members/{userId}` (200). */
export interface RemoveMemberResult {
  status: string;
  org_id: string;
  user_id: string;
}

/** An active session as returned by `GET /v1/sessions` (200). */
export interface Session {
  session_id: string;
  user_agent: string | null;
  ip: string | null;
  created_at: string;
  expires_at: string;
  /** True for the session the calling token belongs to. */
  current: boolean;
}

/** Response of `GET /v1/sessions` (200). */
export interface SessionList {
  items: Session[];
}

/** Response of `DELETE /v1/sessions/{id}` (200). */
export interface RevokeSessionResult {
  status: string;
  session_id: string;
}

/*
 * ─────────────────────────────────────────────────────────────────────────────────────────────
 * merchant-onboarding — onboard / documents / bank-accounts / contracts / fee-tier (work10).
 * Sourced from `merchant-onboarding/app/api/v1/schemas.py` + `app/domain/models.py`. No response
 * ever exposes the encrypted `account_ref` or plaintext `account_details`.
 * ─────────────────────────────────────────────────────────────────────────────────────────────
 */

/** Merchant business type (`MerchantType`). */
export type MerchantType = 'individual' | 'company' | 'nonprofit';

/** Payout rail of a linked bank account (`Rail`). */
export type Rail = 'mpesa' | 'swift' | 'sepa' | 'ach' | 'crypto';

/** Merchant fee tier (`FeeTier`). */
export type FeeTier = 'standard' | 'startup' | 'enterprise';

/** Request body for `POST /v1/merchants/onboard`. */
export interface OnboardInput {
  org_id: string;
  business_name: string;
  registration_no?: string | null;
  /** ISO 3166-1 alpha-2 country code. */
  country: string;
  type: MerchantType;
}

/** Response of `POST /v1/merchants/onboard` (201). */
export interface OnboardResult {
  merchant_id: string;
  status: string;
}

/** A bank account surfaced by status only — never the ref/details. */
export interface BankAccountSummary {
  bank_account_id: string;
  rail: string;
  currency: string;
  status: string;
  verified_at: string | null;
}

/** Response of `GET /v1/merchants/{id}` (200). */
export interface Merchant {
  merchant_id: string;
  org_id: string;
  business_name: string;
  registration_no: string | null;
  tax_id: string | null;
  country: string;
  type: string;
  status: string;
  fee_tier: string;
  onboarded_at: string | null;
  suspended_at: string | null;
  suspended_reason: string | null;
  bank_accounts: BankAccountSummary[];
}

/** Multipart fields for `POST /v1/merchants/{id}/documents`. Sent as `multipart/form-data`. */
export interface UploadDocumentInput {
  /** The document file (e.g. a `File`/`Blob` in the browser, a `Blob` in Node 18+). */
  file: Blob;
  /** Document kind (e.g. cert of incorporation, tax id). */
  kind: string;
}

/** Response of `POST /v1/merchants/{id}/documents` (201). `status` is always `"UPLOADED"`. */
export interface DocumentResult {
  document_id: string;
  status: string;
}

/** Request body for `POST /v1/merchants/{id}/bank-accounts`. */
export interface AddBankAccountInput {
  rail: Rail;
  /** Plaintext account number / wallet — encrypted server-side immediately, never returned. */
  account_details: string;
  /** ISO 4217 currency code. */
  currency: string;
  /** ISO 3166-1 alpha-2 country code. */
  country: string;
}

/** Response of `POST /v1/merchants/{id}/bank-accounts` (201). */
export interface AddBankAccountResult {
  bank_account_id: string;
  status: string;
}

/** Request body for `POST /v1/merchants/{id}/bank-accounts/{baId}/verify` (body optional). */
export interface VerifyBankAccountInput {
  /** Micro-deposit amounts for ACH/SEPA verification. */
  micro_deposit_amounts?: number[] | null;
}

/** Response of `POST /v1/merchants/{id}/bank-accounts/{baId}/verify` (200). */
export interface VerifyBankAccountResult {
  bank_account_id: string;
  status: string;
}

/** Request body for `POST /v1/merchants/{id}/contracts`. */
export interface AcceptContractInput {
  contract_version: string;
  accepted: boolean;
}

/** A contract acceptance record. */
export interface Contract {
  id: number;
  merchant_id: string;
  version: string;
  accepted_by: string;
  accepted_at: string;
  ip: string | null;
  user_agent: string | null;
}

/** Response of `GET /v1/merchants/{id}/contracts` (200). */
export interface ContractList {
  items: Contract[];
}

/** Response of `GET /v1/merchants/{id}/fee-tier` (200). */
export interface FeeTierResult {
  merchant_id: string;
  tier: string;
  effective_at: string;
}

/** Request body for `PATCH /v1/merchants/{id}/fee-tier` (admin). */
export interface UpdateFeeTierInput {
  tier: FeeTier;
}

/*
 * ─────────────────────────────────────────────────────────────────────────────────────────────
 * compliance-risk — KYC sessions + compliance status (work12). Sourced from
 * `compliance-risk/app/api/v1/schemas.py`. `/v1/risk/evaluate` and the KYC callbacks are internal
 * (trusted-network / HMAC) and intentionally NOT part of this public surface.
 * ─────────────────────────────────────────────────────────────────────────────────────────────
 */

/** A compliance flag raised against a user. */
export interface ComplianceFlag {
  kind: string;
  severity: string;
  raised_at: string | null;
}

/** Response of `GET /v1/compliance/status` (200). */
export interface ComplianceStatus {
  user_id: string;
  kyc_tier: number;
  /** Latest risk score, or null when none computed. */
  risk_score: number | null;
  flags: ComplianceFlag[];
}

/** Request body for `POST /v1/kyc/sessions`. */
export interface CreateKycSessionInput {
  user_id: string;
  /** Requested KYC tier, 1–2. */
  tier_requested: number;
}

/** Response of `POST /v1/kyc/sessions` (201). */
export interface CreateKycSessionResult {
  session_id: string;
  provider_url: string;
}

/*
 * ─────────────────────────────────────────────────────────────────────────────────────────────
 * admin-backoffice — unified search + entity drill-down (work11). Sourced from
 * `admin-backoffice/app/api/v1/schemas.py`. Every endpoint requires an admin RS256 token with MFA
 * and the `support.read` scope (enforced in-service).
 * ─────────────────────────────────────────────────────────────────────────────────────────────
 */

/** A single search hit within a {@link SearchResult} group. */
export interface SearchHit {
  type: string;
  id: string;
  label: string;
  status: string | null;
  /** Free-form secondary fields (e.g. country, amount) for display. */
  secondary: Record<string, string>;
}

/** Response of `GET /v1/admin/search` (200). `groups` is keyed by entity type. */
export interface SearchResult {
  query: string;
  groups: Record<string, SearchHit[]>;
  /** Entity types whose upstream read degraded (partial results). */
  degraded: string[];
}

/** Response of `GET /v1/admin/{users,merchants,paylinks,payments}/{id}` (200). */
export interface EntityResult {
  type: string;
  id: string;
  /** The upstream entity projection (shape varies by `type`). */
  data: Record<string, unknown>;
}

/** The four entity kinds an admin can drill into. */
export type AdminEntityType = 'users' | 'merchants' | 'paylinks' | 'payments';

/*
 * ─────────────────────────────────────────────────────────────────────────────────────────────
 * audit-log-service — read the tamper-evident hash chain (work13). Sourced from
 * `audit-log-service/internal/server/auditlog.go`. The POST intake is internal (X-Internal-Token)
 * and intentionally NOT part of this public surface; only the reads are exposed.
 * ─────────────────────────────────────────────────────────────────────────────────────────────
 */

/** The actor on an audit entry. */
export interface AuditActor {
  id: string | null;
  kind: string;
}

/** A single audit-log entry (newest-first in list results). */
export interface AuditEntry {
  entry_id: number;
  /** RFC 3339 timestamp. */
  occurred_at: string;
  actor: AuditActor;
  action: string;
  resource: string;
  /** Prior state, present only when the action carried one. */
  before?: unknown;
  /** New state, present only when the action carried one. */
  after?: unknown;
  /** Opaque context JSON. */
  context: unknown;
  /** Hex-encoded hash of the previous entry. */
  prev_hash: string;
  /** Hex-encoded hash of this entry. */
  entry_hash: string;
}

/** Query parameters for `GET /v1/audit-log`. */
export interface AuditListParams {
  /** Filter by actor UUID. */
  actor?: string;
  /** Filter by resource string. */
  resource?: string;
  /** RFC 3339 lower bound (inclusive). */
  from?: string;
  /** RFC 3339 upper bound (inclusive). */
  to?: string;
  /** Opaque pagination cursor from a previous page's `next_cursor`. */
  cursor?: string;
  /** Page size, 1–100. Defaults to 20 server-side. */
  limit?: number;
}

/** Response of `GET /v1/audit-log` (200). */
export interface AuditList {
  items: AuditEntry[];
  next_cursor: string | null;
}

/** Response of `GET /v1/audit-log/{entry_id}` (200) — the entry plus its inclusion proof. */
export interface AuditEntryWithProof {
  entry: AuditEntry;
  /** Inclusion proof (opaque). */
  proof: unknown;
}

/** Query parameters for `GET /v1/audit-log/verify`. */
export interface VerifyChainParams {
  from?: string;
  to?: string;
}

/** Response of `GET /v1/audit-log/verify` (200). */
export interface VerifyChainResult {
  ok: boolean;
  /** Entry id where the chain first broke, present only when `ok` is false. */
  broken_at?: number;
}
