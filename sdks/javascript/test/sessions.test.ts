import { describe, it, expect } from 'vitest';

import { LinkMintClient } from '../src/client';
import { NotFoundError } from '../src/errors';
import { createMockFetch } from './helpers/mockFetch';

const BASE = 'https://api.linkmint.test';

describe('sessions.list', () => {
  it('GETs /v1/sessions and returns the typed list', async () => {
    const mock = createMockFetch({
      status: 200,
      body: {
        items: [
          {
            session_id: 's-1',
            user_agent: 'curl/8',
            ip: '127.0.0.1',
            created_at: '2026-01-01T00:00:00Z',
            expires_at: '2026-01-02T00:00:00Z',
            current: true,
          },
        ],
      },
    });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    const result = await client.sessions.list();

    expect(result.items[0]?.current).toBe(true);
    const call = mock.lastCall();
    expect(call.method).toBe('GET');
    expect(call.url).toBe(`${BASE}/v1/sessions`);
    expect(call.headers['Idempotency-Key']).toBeUndefined();
    expect(call.headers['X-Creator-Addr']).toBeUndefined();
  });
});

describe('sessions.revoke', () => {
  it('DELETEs /v1/sessions/{id} with an idempotency key', async () => {
    const mock = createMockFetch({ status: 200, body: { status: 'revoked', session_id: 's-1' } });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    const result = await client.sessions.revoke('s-1');
    expect(result).toEqual({ status: 'revoked', session_id: 's-1' });
    const call = mock.lastCall();
    expect(call.method).toBe('DELETE');
    expect(call.url).toBe(`${BASE}/v1/sessions/s-1`);
    expect(call.headers['Idempotency-Key']).toBeTypeOf('string');
  });

  it('maps a 404 NOT_FOUND to NotFoundError', async () => {
    const mock = createMockFetch({
      status: 404,
      body: { error: { code: 'NOT_FOUND', message: 'no such session' } },
    });
    const client = new LinkMintClient({ baseUrl: BASE, fetch: mock.fetch });

    await expect(client.sessions.revoke('missing')).rejects.toBeInstanceOf(NotFoundError);
  });
});
