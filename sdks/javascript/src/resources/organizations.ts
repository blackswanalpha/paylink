/**
 * `/v1/organizations` resource — create an org (creator becomes owner) and manage members
 * (identity-service, work09). All calls require a bearer token; RBAC is enforced server-side.
 */

import type { HttpClient, RequestOptions } from '../http';
import type {
  AddMemberInput,
  CreateOrgInput,
  Member,
  MemberList,
  Org,
  OrgList,
  RemoveMemberResult,
} from '../types';

export class OrganizationsResource {
  constructor(private readonly http: HttpClient) {}

  /** List the caller's organizations (newest first). `GET /v1/organizations` → 200. */
  list(options: RequestOptions = {}): Promise<OrgList> {
    return this.http.request<OrgList>({ method: 'GET', path: '/v1/organizations' }, options);
  }

  /** Create an organization. `POST /v1/organizations` → 201. */
  create(input: CreateOrgInput, options: RequestOptions = {}): Promise<Org> {
    return this.http.request<Org>(
      {
        method: 'POST',
        path: '/v1/organizations',
        body: { name: input.name, type: input.type },
        idempotencyKey: options.idempotencyKey ?? this.http.newIdempotencyKey(),
      },
      options,
    );
  }

  /** Add a member to an org. `POST /v1/organizations/{orgId}/members` → 201. */
  addMember(orgId: string, input: AddMemberInput, options: RequestOptions = {}): Promise<Member> {
    const body: Record<string, unknown> = { role: input.role };
    if (input.user_id !== undefined) {
      body.user_id = input.user_id;
    }
    if (input.email !== undefined) {
      body.email = input.email;
    }
    return this.http.request<Member>(
      {
        method: 'POST',
        path: `/v1/organizations/${encodeURIComponent(orgId)}/members`,
        body,
        idempotencyKey: options.idempotencyKey ?? this.http.newIdempotencyKey(),
      },
      options,
    );
  }

  /** List an org's members. `GET /v1/organizations/{orgId}/members` → 200. */
  listMembers(orgId: string, options: RequestOptions = {}): Promise<MemberList> {
    return this.http.request<MemberList>(
      { method: 'GET', path: `/v1/organizations/${encodeURIComponent(orgId)}/members` },
      options,
    );
  }

  /** Remove a member from an org. `DELETE /v1/organizations/{orgId}/members/{userId}` → 200. */
  removeMember(
    orgId: string,
    userId: string,
    options: RequestOptions = {},
  ): Promise<RemoveMemberResult> {
    return this.http.request<RemoveMemberResult>(
      {
        method: 'DELETE',
        path: `/v1/organizations/${encodeURIComponent(orgId)}/members/${encodeURIComponent(userId)}`,
        idempotencyKey: options.idempotencyKey ?? this.http.newIdempotencyKey(),
      },
      options,
    );
  }
}
