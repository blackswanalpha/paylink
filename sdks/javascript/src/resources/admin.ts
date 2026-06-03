/**
 * `/v1/admin` resource — unified search and entity drill-down for the ops console (admin-backoffice,
 * work11). Every call requires an admin RS256 token with MFA and the `support.read` scope, enforced
 * in-service. Read-only (Phase 1).
 */

import type { HttpClient, RequestOptions } from '../http';
import type { AdminEntityType, EntityResult, SearchResult } from '../types';

export class AdminResource {
  constructor(private readonly http: HttpClient) {}

  /** Unified search across users, merchants, PayLinks, and payments. `GET /v1/admin/search?q=` → 200. */
  search(q: string, options: RequestOptions = {}): Promise<SearchResult> {
    return this.http.request<SearchResult>(
      { method: 'GET', path: '/v1/admin/search', query: { q } },
      options,
    );
  }

  /** Drill into a single entity. `GET /v1/admin/{type}/{id}` → 200. */
  getEntity(
    type: AdminEntityType,
    id: string,
    options: RequestOptions = {},
  ): Promise<EntityResult> {
    return this.http.request<EntityResult>(
      { method: 'GET', path: `/v1/admin/${type}/${encodeURIComponent(id)}` },
      options,
    );
  }

  /** Drill into a user. `GET /v1/admin/users/{id}` → 200. */
  getUser(id: string, options: RequestOptions = {}): Promise<EntityResult> {
    return this.getEntity('users', id, options);
  }

  /** Drill into a merchant. `GET /v1/admin/merchants/{id}` → 200. */
  getMerchant(id: string, options: RequestOptions = {}): Promise<EntityResult> {
    return this.getEntity('merchants', id, options);
  }

  /** Drill into a PayLink. `GET /v1/admin/paylinks/{id}` → 200. */
  getPaylink(id: string, options: RequestOptions = {}): Promise<EntityResult> {
    return this.getEntity('paylinks', id, options);
  }

  /** Drill into a payment. `GET /v1/admin/payments/{id}` → 200. */
  getPayment(id: string, options: RequestOptions = {}): Promise<EntityResult> {
    return this.getEntity('payments', id, options);
  }
}
