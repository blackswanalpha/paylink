/**
 * `/v1/auth/*` resource — register, login, refresh, logout, OAuth, and MFA (identity-service,
 * work09). `register`/`login`/`refresh`/`oauthStart`/`oauthCallback` are public (mint or exchange
 * credentials); `logout` and the `mfa*` calls require a bearer token. Mutations carry an
 * auto-`Idempotency-Key` like the rest of the SDK; the client never sends `X-Creator-Addr`.
 */

import type { HttpClient, RequestOptions } from '../http';
import type {
  LoginInput,
  LogoutInput,
  LogoutResult,
  MfaCodeInput,
  MfaDisableResult,
  MfaEnrollResult,
  MfaVerifyResult,
  OAuthCallbackInput,
  OAuthStartInput,
  OAuthStartResult,
  PasswordResetConfirmInput,
  PasswordResetConfirmResult,
  PasswordResetRequestInput,
  PasswordResetRequestResult,
  RefreshInput,
  RegisterInput,
  RegisterResult,
  TokenResponse,
} from '../types';

export class AuthResource {
  constructor(private readonly http: HttpClient) {}

  /** Register a new user. `POST /v1/auth/register` → 201. One of `email`/`phone` is required. */
  register(input: RegisterInput, options: RequestOptions = {}): Promise<RegisterResult> {
    const body: Record<string, unknown> = { password: input.password };
    if (input.email !== undefined) {
      body.email = input.email;
    }
    if (input.phone !== undefined) {
      body.phone = input.phone;
    }
    return this.http.request<RegisterResult>(
      {
        method: 'POST',
        path: '/v1/auth/register',
        body,
        idempotencyKey: options.idempotencyKey ?? this.http.newIdempotencyKey(),
      },
      options,
    );
  }

  /** Exchange credentials for a token pair. `POST /v1/auth/login` → 200. */
  login(input: LoginInput, options: RequestOptions = {}): Promise<TokenResponse> {
    const body: Record<string, unknown> = { password: input.password };
    if (input.email !== undefined) {
      body.email = input.email;
    }
    if (input.phone !== undefined) {
      body.phone = input.phone;
    }
    if (input.mfa_code !== undefined) {
      body.mfa_code = input.mfa_code;
    }
    return this.http.request<TokenResponse>(
      {
        method: 'POST',
        path: '/v1/auth/login',
        body,
        idempotencyKey: options.idempotencyKey ?? this.http.newIdempotencyKey(),
      },
      options,
    );
  }

  /** Exchange a refresh token for a fresh token pair. `POST /v1/auth/refresh` → 200. */
  refresh(input: RefreshInput, options: RequestOptions = {}): Promise<TokenResponse> {
    return this.http.request<TokenResponse>(
      {
        method: 'POST',
        path: '/v1/auth/refresh',
        body: { refresh_token: input.refresh_token },
        idempotencyKey: options.idempotencyKey ?? this.http.newIdempotencyKey(),
      },
      options,
    );
  }

  /** Revoke a refresh token (requires a bearer token). `POST /v1/auth/logout` → 200. */
  logout(input: LogoutInput, options: RequestOptions = {}): Promise<LogoutResult> {
    return this.http.request<LogoutResult>(
      {
        method: 'POST',
        path: '/v1/auth/logout',
        body: { refresh_token: input.refresh_token },
        idempotencyKey: options.idempotencyKey ?? this.http.newIdempotencyKey(),
      },
      options,
    );
  }

  /** Begin an OAuth flow. `POST /v1/auth/oauth/{provider}/start` → 200. */
  oauthStart(
    provider: string,
    input: OAuthStartInput = {},
    options: RequestOptions = {},
  ): Promise<OAuthStartResult> {
    const body: Record<string, unknown> = {};
    if (input.redirect_uri !== undefined) {
      body.redirect_uri = input.redirect_uri;
    }
    if (input.state !== undefined) {
      body.state = input.state;
    }
    return this.http.request<OAuthStartResult>(
      {
        method: 'POST',
        path: `/v1/auth/oauth/${encodeURIComponent(provider)}/start`,
        body,
        idempotencyKey: options.idempotencyKey ?? this.http.newIdempotencyKey(),
      },
      options,
    );
  }

  /** Complete an OAuth flow, exchanging the code for tokens. `POST /v1/auth/oauth/{provider}/callback` → 200. */
  oauthCallback(
    provider: string,
    input: OAuthCallbackInput,
    options: RequestOptions = {},
  ): Promise<TokenResponse> {
    const body: Record<string, unknown> = { code: input.code };
    if (input.state !== undefined) {
      body.state = input.state;
    }
    if (input.redirect_uri !== undefined) {
      body.redirect_uri = input.redirect_uri;
    }
    return this.http.request<TokenResponse>(
      {
        method: 'POST',
        path: `/v1/auth/oauth/${encodeURIComponent(provider)}/callback`,
        body,
        idempotencyKey: options.idempotencyKey ?? this.http.newIdempotencyKey(),
      },
      options,
    );
  }

  /**
   * Request a password reset (public). `POST /v1/auth/password/reset-request` → 200.
   * Always succeeds with the same shape whether or not the account exists (anti-enumeration).
   */
  requestPasswordReset(
    input: PasswordResetRequestInput,
    options: RequestOptions = {},
  ): Promise<PasswordResetRequestResult> {
    const body: Record<string, unknown> = {};
    if (input.email !== undefined) {
      body.email = input.email;
    }
    if (input.phone !== undefined) {
      body.phone = input.phone;
    }
    return this.http.request<PasswordResetRequestResult>(
      {
        method: 'POST',
        path: '/v1/auth/password/reset-request',
        body,
        idempotencyKey: options.idempotencyKey ?? this.http.newIdempotencyKey(),
      },
      options,
    );
  }

  /** Complete a password reset with a reset token (public). `POST /v1/auth/password/reset-confirm` → 200. */
  confirmPasswordReset(
    input: PasswordResetConfirmInput,
    options: RequestOptions = {},
  ): Promise<PasswordResetConfirmResult> {
    return this.http.request<PasswordResetConfirmResult>(
      {
        method: 'POST',
        path: '/v1/auth/password/reset-confirm',
        body: { token: input.token, new_password: input.new_password },
        idempotencyKey: options.idempotencyKey ?? this.http.newIdempotencyKey(),
      },
      options,
    );
  }

  /** Begin TOTP MFA enrollment (requires a bearer token). `POST /v1/auth/mfa/enroll` → 200. */
  mfaEnroll(options: RequestOptions = {}): Promise<MfaEnrollResult> {
    return this.http.request<MfaEnrollResult>(
      {
        method: 'POST',
        path: '/v1/auth/mfa/enroll',
        idempotencyKey: options.idempotencyKey ?? this.http.newIdempotencyKey(),
      },
      options,
    );
  }

  /** Verify a TOTP code to activate MFA. `POST /v1/auth/mfa/verify` → 200. */
  mfaVerify(input: MfaCodeInput, options: RequestOptions = {}): Promise<MfaVerifyResult> {
    return this.http.request<MfaVerifyResult>(
      {
        method: 'POST',
        path: '/v1/auth/mfa/verify',
        body: { code: input.code },
        idempotencyKey: options.idempotencyKey ?? this.http.newIdempotencyKey(),
      },
      options,
    );
  }

  /** Disable MFA with a current TOTP code. `POST /v1/auth/mfa/disable` → 200. */
  mfaDisable(input: MfaCodeInput, options: RequestOptions = {}): Promise<MfaDisableResult> {
    return this.http.request<MfaDisableResult>(
      {
        method: 'POST',
        path: '/v1/auth/mfa/disable',
        body: { code: input.code },
        idempotencyKey: options.idempotencyKey ?? this.http.newIdempotencyKey(),
      },
      options,
    );
  }
}
