/**
 * `@linkmint/sdk` — typed TypeScript client for the LinkMint `/v1` API.
 *
 * Resources: paylinks, payments, notifications, and (work08) auth, users, organizations, sessions,
 * merchants, compliance, admin, and auditLog. See {@link LinkMintClient}. The client talks only to
 * the LinkMint API gateway and is rail-agnostic (invariant A.4): it exposes no rail-specific PayLink
 * fields and never sends `X-Creator-Addr` (the gateway derives identity from the bearer token).
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
export { AuthResource } from './resources/auth';
export { UsersResource } from './resources/users';
export { OrganizationsResource } from './resources/organizations';
export { SessionsResource } from './resources/sessions';
export { MerchantsResource } from './resources/merchants';
export { ComplianceResource } from './resources/compliance';
export { AdminResource } from './resources/admin';
export { AuditLogResource } from './resources/auditLog';

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
  NotificationChannel,
  NotificationEventKind,
  NotificationPreferences,
  UpdateNotificationPreferencesInput,
  // identity-service (work09)
  OrgType,
  Role,
  Scope,
  RegisterInput,
  RegisterResult,
  LoginInput,
  TokenResponse,
  RefreshInput,
  LogoutInput,
  LogoutResult,
  OAuthStartInput,
  OAuthStartResult,
  OAuthCallbackInput,
  MfaEnrollResult,
  MfaCodeInput,
  MfaVerifyResult,
  MfaDisableResult,
  PasswordResetRequestInput,
  PasswordResetRequestResult,
  PasswordResetConfirmInput,
  PasswordResetConfirmResult,
  OrgRoleEntry,
  UserProfile,
  UpdateProfileInput,
  IssueApiKeyInput,
  IssueApiKeyResult,
  ApiKey,
  ApiKeyList,
  RevokeApiKeyResult,
  CreateOrgInput,
  Org,
  OrgList,
  AddMemberInput,
  Member,
  MemberList,
  RemoveMemberResult,
  Session,
  SessionList,
  RevokeSessionResult,
  // merchant-onboarding (work10)
  MerchantType,
  Rail,
  FeeTier,
  OnboardInput,
  OnboardResult,
  BankAccountSummary,
  Merchant,
  UploadDocumentInput,
  DocumentResult,
  AddBankAccountInput,
  AddBankAccountResult,
  VerifyBankAccountInput,
  VerifyBankAccountResult,
  AcceptContractInput,
  Contract,
  ContractList,
  FeeTierResult,
  UpdateFeeTierInput,
  // compliance-risk (work12)
  ComplianceFlag,
  ComplianceStatus,
  CreateKycSessionInput,
  CreateKycSessionResult,
  // admin-backoffice (work11)
  SearchHit,
  SearchResult,
  EntityResult,
  AdminEntityType,
  // audit-log-service (work13)
  AuditActor,
  AuditEntry,
  AuditListParams,
  AuditList,
  AuditEntryWithProof,
  VerifyChainParams,
  VerifyChainResult,
} from './types';
