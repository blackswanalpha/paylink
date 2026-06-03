/**
 * `/v1/audit-log` resource — read the tamper-evident hash chain (audit-log-service, work13). Reads
 * require an admin/compliance RS256 token (verified in-service when configured). The POST intake is
 * internal (X-Internal-Token) and intentionally not exposed here.
 */

import type { HttpClient, RequestOptions } from '../http';
import type {
  AuditEntryWithProof,
  AuditList,
  AuditListParams,
  VerifyChainParams,
  VerifyChainResult,
} from '../types';

export class AuditLogResource {
  constructor(private readonly http: HttpClient) {}

  /** Query audit entries (newest-first) with cursor pagination. `GET /v1/audit-log` → 200. */
  list(params: AuditListParams = {}, options: RequestOptions = {}): Promise<AuditList> {
    return this.http.request<AuditList>(
      {
        method: 'GET',
        path: '/v1/audit-log',
        query: {
          actor: params.actor,
          resource: params.resource,
          from: params.from,
          to: params.to,
          cursor: params.cursor,
          limit: params.limit,
        },
      },
      options,
    );
  }

  /** Fetch a single entry plus its inclusion proof. `GET /v1/audit-log/{entry_id}` → 200. */
  get(entryId: number | string, options: RequestOptions = {}): Promise<AuditEntryWithProof> {
    return this.http.request<AuditEntryWithProof>(
      { method: 'GET', path: `/v1/audit-log/${encodeURIComponent(String(entryId))}` },
      options,
    );
  }

  /** Verify the chain over an optional time range. `GET /v1/audit-log/verify` → 200. */
  verify(params: VerifyChainParams = {}, options: RequestOptions = {}): Promise<VerifyChainResult> {
    return this.http.request<VerifyChainResult>(
      { method: 'GET', path: '/v1/audit-log/verify', query: { from: params.from, to: params.to } },
      options,
    );
  }
}
