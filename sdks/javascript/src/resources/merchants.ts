/**
 * `/v1/merchants` resource — onboard a merchant and manage its documents, bank accounts, contracts,
 * and fee tier (merchant-onboarding, work10). All calls require a bearer token; access is scoped to
 * the caller's org membership server-side. Responses never expose bank-account secrets.
 */

import type { HttpClient, RequestOptions } from '../http';
import type {
  AcceptContractInput,
  AddBankAccountInput,
  AddBankAccountResult,
  Contract,
  ContractList,
  DocumentResult,
  FeeTierResult,
  Merchant,
  OnboardInput,
  OnboardResult,
  UpdateFeeTierInput,
  UploadDocumentInput,
  VerifyBankAccountInput,
  VerifyBankAccountResult,
} from '../types';

export class MerchantsResource {
  constructor(private readonly http: HttpClient) {}

  /** Onboard a merchant (created in DRAFT). `POST /v1/merchants/onboard` → 201. */
  onboard(input: OnboardInput, options: RequestOptions = {}): Promise<OnboardResult> {
    const body: Record<string, unknown> = {
      org_id: input.org_id,
      business_name: input.business_name,
      country: input.country,
      type: input.type,
    };
    if (input.registration_no !== undefined) {
      body.registration_no = input.registration_no;
    }
    return this.http.request<OnboardResult>(
      {
        method: 'POST',
        path: '/v1/merchants/onboard',
        body,
        idempotencyKey: options.idempotencyKey ?? this.http.newIdempotencyKey(),
      },
      options,
    );
  }

  /** Fetch a merchant's full record. `GET /v1/merchants/{id}` → 200. */
  get(merchantId: string, options: RequestOptions = {}): Promise<Merchant> {
    return this.http.request<Merchant>(
      { method: 'GET', path: `/v1/merchants/${encodeURIComponent(merchantId)}` },
      options,
    );
  }

  /** Fetch a merchant's current fee tier. `GET /v1/merchants/{id}/fee-tier` → 200. */
  feeTier(merchantId: string, options: RequestOptions = {}): Promise<FeeTierResult> {
    return this.http.request<FeeTierResult>(
      { method: 'GET', path: `/v1/merchants/${encodeURIComponent(merchantId)}/fee-tier` },
      options,
    );
  }

  /** Change a merchant's fee tier (admin). `PATCH /v1/merchants/{id}/fee-tier` → 200. */
  updateFeeTier(
    merchantId: string,
    input: UpdateFeeTierInput,
    options: RequestOptions = {},
  ): Promise<FeeTierResult> {
    return this.http.request<FeeTierResult>(
      {
        method: 'PATCH',
        path: `/v1/merchants/${encodeURIComponent(merchantId)}/fee-tier`,
        body: { tier: input.tier },
        idempotencyKey: options.idempotencyKey ?? this.http.newIdempotencyKey(),
      },
      options,
    );
  }

  /**
   * Upload a verification document (multipart). `POST /v1/merchants/{id}/documents` → 201.
   * Sent as `multipart/form-data` with `file` + `kind` fields.
   */
  addDocument(
    merchantId: string,
    input: UploadDocumentInput,
    options: RequestOptions = {},
  ): Promise<DocumentResult> {
    const form = new FormData();
    form.append('file', input.file);
    form.append('kind', input.kind);
    return this.http.request<DocumentResult>(
      {
        method: 'POST',
        path: `/v1/merchants/${encodeURIComponent(merchantId)}/documents`,
        body: form,
        idempotencyKey: options.idempotencyKey ?? this.http.newIdempotencyKey(),
      },
      options,
    );
  }

  /** Link a bank account. `POST /v1/merchants/{id}/bank-accounts` → 201. */
  addBankAccount(
    merchantId: string,
    input: AddBankAccountInput,
    options: RequestOptions = {},
  ): Promise<AddBankAccountResult> {
    return this.http.request<AddBankAccountResult>(
      {
        method: 'POST',
        path: `/v1/merchants/${encodeURIComponent(merchantId)}/bank-accounts`,
        body: {
          rail: input.rail,
          account_details: input.account_details,
          currency: input.currency,
          country: input.country,
        },
        idempotencyKey: options.idempotencyKey ?? this.http.newIdempotencyKey(),
      },
      options,
    );
  }

  /** Verify a linked bank account. `POST /v1/merchants/{id}/bank-accounts/{baId}/verify` → 200. */
  verifyBankAccount(
    merchantId: string,
    bankAccountId: string,
    input: VerifyBankAccountInput = {},
    options: RequestOptions = {},
  ): Promise<VerifyBankAccountResult> {
    const body: Record<string, unknown> = {};
    if (input.micro_deposit_amounts !== undefined) {
      body.micro_deposit_amounts = input.micro_deposit_amounts;
    }
    return this.http.request<VerifyBankAccountResult>(
      {
        method: 'POST',
        path: `/v1/merchants/${encodeURIComponent(merchantId)}/bank-accounts/${encodeURIComponent(bankAccountId)}/verify`,
        body,
        idempotencyKey: options.idempotencyKey ?? this.http.newIdempotencyKey(),
      },
      options,
    );
  }

  /** Accept a contract version. `POST /v1/merchants/{id}/contracts` → 201. */
  acceptContract(
    merchantId: string,
    input: AcceptContractInput,
    options: RequestOptions = {},
  ): Promise<Contract> {
    return this.http.request<Contract>(
      {
        method: 'POST',
        path: `/v1/merchants/${encodeURIComponent(merchantId)}/contracts`,
        body: { contract_version: input.contract_version, accepted: input.accepted },
        idempotencyKey: options.idempotencyKey ?? this.http.newIdempotencyKey(),
      },
      options,
    );
  }

  /** List a merchant's accepted contracts. `GET /v1/merchants/{id}/contracts` → 200. */
  listContracts(merchantId: string, options: RequestOptions = {}): Promise<ContractList> {
    return this.http.request<ContractList>(
      { method: 'GET', path: `/v1/merchants/${encodeURIComponent(merchantId)}/contracts` },
      options,
    );
  }
}
