/**
 * `/v1/compliance` + `/v1/kyc` resource — read a user's KYC/risk status and start a KYC session
 * (compliance-risk, work12). Both require a bearer token and are self-or-admin scoped server-side.
 * The internal `/v1/risk/evaluate` and provider callbacks are intentionally not exposed here.
 */

import type { HttpClient, RequestOptions } from '../http';
import type { ComplianceStatus, CreateKycSessionInput, CreateKycSessionResult } from '../types';

export class ComplianceResource {
  constructor(private readonly http: HttpClient) {}

  /**
   * Read a user's compliance status (KYC tier, latest risk score, open flags).
   * `GET /v1/compliance/status?user_id=...` → 200. A user may read only their own status.
   */
  status(userId: string, options: RequestOptions = {}): Promise<ComplianceStatus> {
    return this.http.request<ComplianceStatus>(
      { method: 'GET', path: '/v1/compliance/status', query: { user_id: userId } },
      options,
    );
  }

  /** Start a KYC session for a user. `POST /v1/kyc/sessions` → 201. */
  createKycSession(
    input: CreateKycSessionInput,
    options: RequestOptions = {},
  ): Promise<CreateKycSessionResult> {
    return this.http.request<CreateKycSessionResult>(
      {
        method: 'POST',
        path: '/v1/kyc/sessions',
        body: { user_id: input.user_id, tier_requested: input.tier_requested },
        idempotencyKey: options.idempotencyKey ?? this.http.newIdempotencyKey(),
      },
      options,
    );
  }
}
