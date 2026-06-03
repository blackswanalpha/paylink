/**
 * `/v1/sessions` resource — list the authenticated user's active sessions and revoke one
 * (identity-service, work09). Both calls require a bearer token; sessions are scoped server-side.
 */

import type { HttpClient, RequestOptions } from '../http';
import type { RevokeSessionResult, SessionList } from '../types';

export class SessionsResource {
  constructor(private readonly http: HttpClient) {}

  /** List the caller's active sessions. `GET /v1/sessions` → 200. */
  list(options: RequestOptions = {}): Promise<SessionList> {
    return this.http.request<SessionList>({ method: 'GET', path: '/v1/sessions' }, options);
  }

  /** Revoke a session. `DELETE /v1/sessions/{id}` → 200. */
  revoke(sessionId: string, options: RequestOptions = {}): Promise<RevokeSessionResult> {
    return this.http.request<RevokeSessionResult>(
      {
        method: 'DELETE',
        path: `/v1/sessions/${encodeURIComponent(sessionId)}`,
        idempotencyKey: options.idempotencyKey ?? this.http.newIdempotencyKey(),
      },
      options,
    );
  }
}
