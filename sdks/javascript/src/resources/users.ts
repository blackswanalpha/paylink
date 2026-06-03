/**
 * `/v1/users/me` resource — the authenticated user's profile and scoped API keys (identity-service,
 * work09). All calls require a bearer token; the caller is resolved server-side from the token's
 * `sub`, so no id is ever passed from the client.
 */

import type { HttpClient, RequestOptions } from '../http';
import type {
  ApiKeyList,
  IssueApiKeyInput,
  IssueApiKeyResult,
  RevokeApiKeyResult,
  UpdateProfileInput,
  UserProfile,
} from '../types';

export class UsersResource {
  constructor(private readonly http: HttpClient) {}

  /** Fetch the authenticated user's profile. `GET /v1/users/me` → 200. */
  me(options: RequestOptions = {}): Promise<UserProfile> {
    return this.http.request<UserProfile>({ method: 'GET', path: '/v1/users/me' }, options);
  }

  /** Update the authenticated user's profile. `PATCH /v1/users/me` → 200. */
  updateMe(input: UpdateProfileInput, options: RequestOptions = {}): Promise<UserProfile> {
    const body: Record<string, unknown> = {};
    if (input.email !== undefined) {
      body.email = input.email;
    }
    if (input.phone !== undefined) {
      body.phone = input.phone;
    }
    return this.http.request<UserProfile>(
      {
        method: 'PATCH',
        path: '/v1/users/me',
        body,
        idempotencyKey: options.idempotencyKey ?? this.http.newIdempotencyKey(),
      },
      options,
    );
  }

  /**
   * Issue a scoped API key. `POST /v1/users/me/api-keys` → 201.
   * The response's `full_key` is the only time the secret is returned; persist it immediately.
   */
  createApiKey(input: IssueApiKeyInput, options: RequestOptions = {}): Promise<IssueApiKeyResult> {
    const body: Record<string, unknown> = { org_id: input.org_id, name: input.name };
    if (input.scopes !== undefined) {
      body.scopes = input.scopes;
    }
    return this.http.request<IssueApiKeyResult>(
      {
        method: 'POST',
        path: '/v1/users/me/api-keys',
        body,
        idempotencyKey: options.idempotencyKey ?? this.http.newIdempotencyKey(),
      },
      options,
    );
  }

  /** List the authenticated user's API keys (no secret material). `GET /v1/users/me/api-keys` → 200. */
  listApiKeys(options: RequestOptions = {}): Promise<ApiKeyList> {
    return this.http.request<ApiKeyList>({ method: 'GET', path: '/v1/users/me/api-keys' }, options);
  }

  /** Revoke an API key. `DELETE /v1/users/me/api-keys/{id}` → 200. */
  revokeApiKey(apiKeyId: string, options: RequestOptions = {}): Promise<RevokeApiKeyResult> {
    return this.http.request<RevokeApiKeyResult>(
      {
        method: 'DELETE',
        path: `/v1/users/me/api-keys/${encodeURIComponent(apiKeyId)}`,
        idempotencyKey: options.idempotencyKey ?? this.http.newIdempotencyKey(),
      },
      options,
    );
  }
}
