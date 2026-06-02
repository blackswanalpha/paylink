/** `/v1/notifications` resource — the in-app notification center (list / mark read / mark all read). */

import type { HttpClient, RequestOptions } from '../http';
import type {
  ListNotificationsParams,
  MarkAllReadResult,
  Notification,
  NotificationList,
} from '../types';

export class NotificationsResource {
  constructor(private readonly http: HttpClient) {}

  /**
   * List the caller's notifications, newest first, with cursor pagination.
   * `GET /v1/notifications` → 200. Scoped server-side to the authenticated caller.
   */
  list(
    params: ListNotificationsParams = {},
    options: RequestOptions = {},
  ): Promise<NotificationList> {
    return this.http.request<NotificationList>(
      {
        method: 'GET',
        path: '/v1/notifications',
        query: { limit: params.limit, cursor: params.cursor },
      },
      options,
    );
  }

  /**
   * Mark one notification read (idempotent). `POST /v1/notifications/{id}/read` → 200.
   * An `Idempotency-Key` is generated automatically unless supplied via `options`.
   */
  markRead(id: string, options: RequestOptions = {}): Promise<Notification> {
    return this.http.request<Notification>(
      {
        method: 'POST',
        path: `/v1/notifications/${encodeURIComponent(id)}/read`,
        idempotencyKey: options.idempotencyKey ?? this.http.newIdempotencyKey(),
      },
      options,
    );
  }

  /**
   * Mark all of the caller's notifications read. `POST /v1/notifications/read-all` → 200.
   * An `Idempotency-Key` is generated automatically unless supplied via `options`.
   */
  markAllRead(options: RequestOptions = {}): Promise<MarkAllReadResult> {
    return this.http.request<MarkAllReadResult>(
      {
        method: 'POST',
        path: '/v1/notifications/read-all',
        idempotencyKey: options.idempotencyKey ?? this.http.newIdempotencyKey(),
      },
      options,
    );
  }
}
