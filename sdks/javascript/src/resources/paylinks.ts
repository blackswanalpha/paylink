/** `/v1/paylinks` resource — create / get / list / cancel. */

import type { HttpClient, RequestOptions } from '../http';
import type {
  CancelPayLinkResult,
  CreatePayLinkInput,
  CreatePayLinkResult,
  ListPayLinksParams,
  PayLink,
  PayLinkList,
} from '../types';

function toIso(value: string | Date): string {
  return value instanceof Date ? value.toISOString() : value;
}

export class PayLinksResource {
  constructor(private readonly http: HttpClient) {}

  /**
   * Create a PayLink. `POST /v1/paylinks` → 201.
   * An `Idempotency-Key` is generated automatically unless supplied via `options`.
   */
  create(input: CreatePayLinkInput, options: RequestOptions = {}): Promise<CreatePayLinkResult> {
    const body: Record<string, unknown> = {
      receiver: input.receiver,
      amount: input.amount,
      expiry: toIso(input.expiry),
    };
    if (input.currency !== undefined) {
      body.currency = input.currency;
    }
    if (input.usage !== undefined) {
      body.usage = input.usage;
    }
    if (input.metadata !== undefined) {
      body.metadata = input.metadata;
    }
    if (input.rules !== undefined) {
      body.rules = input.rules;
    }
    return this.http.request<CreatePayLinkResult>(
      {
        method: 'POST',
        path: '/v1/paylinks',
        body,
        idempotencyKey: options.idempotencyKey ?? this.http.newIdempotencyKey(),
      },
      options,
    );
  }

  /** Fetch a PayLink by id. `GET /v1/paylinks/{pl_id}` → 200. */
  get(plId: string, options: RequestOptions = {}): Promise<PayLink> {
    return this.http.request<PayLink>(
      { method: 'GET', path: `/v1/paylinks/${encodeURIComponent(plId)}` },
      options,
    );
  }

  /** List PayLinks with optional filters and cursor pagination. `GET /v1/paylinks` → 200. */
  list(params: ListPayLinksParams = {}, options: RequestOptions = {}): Promise<PayLinkList> {
    return this.http.request<PayLinkList>(
      {
        method: 'GET',
        path: '/v1/paylinks',
        query: {
          creator: params.creator,
          receiver: params.receiver,
          status: params.status,
          limit: params.limit,
          cursor: params.cursor,
        },
      },
      options,
    );
  }

  /**
   * Cancel a PayLink. `POST /v1/paylinks/{pl_id}/cancel` → 200.
   * An `Idempotency-Key` is generated automatically unless supplied via `options`.
   */
  cancel(plId: string, options: RequestOptions = {}): Promise<CancelPayLinkResult> {
    return this.http.request<CancelPayLinkResult>(
      {
        method: 'POST',
        path: `/v1/paylinks/${encodeURIComponent(plId)}/cancel`,
        idempotencyKey: options.idempotencyKey ?? this.http.newIdempotencyKey(),
      },
      options,
    );
  }
}
