/** `/v1/payments` resource — initiate / get status. */

import type { HttpClient, RequestOptions } from '../http';
import type { InitiatePaymentInput, Payment } from '../types';

export class PaymentsResource {
  constructor(private readonly http: HttpClient) {}

  /**
   * Initiate a payment against a PayLink. `POST /v1/payments` → 201.
   * `Idempotency-Key` is required by the orchestrator; the SDK generates one automatically unless
   * supplied via `options`. `rail` is an opaque routing label (invariant A.4).
   */
  initiate(input: InitiatePaymentInput, options: RequestOptions = {}): Promise<Payment> {
    return this.http.request<Payment>(
      {
        method: 'POST',
        path: '/v1/payments',
        body: { paylink_id: input.paylink_id, rail: input.rail },
        idempotencyKey: options.idempotencyKey ?? this.http.newIdempotencyKey(),
      },
      options,
    );
  }

  /**
   * Get a payment and its current status. `GET /v1/payments/{id}` → 200.
   * The orchestrator reconciles the status against on-chain truth before responding.
   */
  get(id: string, options: RequestOptions = {}): Promise<Payment> {
    return this.http.request<Payment>(
      { method: 'GET', path: `/v1/payments/${encodeURIComponent(id)}` },
      options,
    );
  }
}
