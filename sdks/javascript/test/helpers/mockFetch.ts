/** A faithful mock `fetch` that records requests and returns real `Response` objects. */

import type { FetchLike } from '../../src/http';

type FetchInput = Parameters<typeof fetch>[0];
type FetchInit = NonNullable<Parameters<typeof fetch>[1]>;

export interface RecordedRequest {
  url: string;
  method: string;
  headers: Record<string, string>;
  body: unknown;
}

export interface MockResponseSpec {
  status?: number;
  /** JSON-serialized when an object; sent verbatim when a string. */
  body?: unknown;
  headers?: Record<string, string>;
  /** A raw (possibly non-JSON) body, overriding `body`. */
  rawBody?: string;
}

export type Responder =
  | MockResponseSpec
  | MockResponseSpec[]
  | ((req: RecordedRequest) => MockResponseSpec | Promise<MockResponseSpec>);

export interface MockFetch {
  fetch: FetchLike;
  calls: RecordedRequest[];
  lastCall(): RecordedRequest;
}

export function createMockFetch(responder: Responder): MockFetch {
  const calls: RecordedRequest[] = [];
  const queue = Array.isArray(responder) ? [...responder] : null;

  const fetchImpl: FetchLike = async (input, init) => {
    const req: RecordedRequest = {
      url: urlOf(input),
      method: init?.method ?? 'GET',
      headers: normalizeHeaders(init?.headers),
      body: parseBody(init?.body),
    };
    calls.push(req);

    let spec: MockResponseSpec;
    if (typeof responder === 'function') {
      spec = await responder(req);
    } else if (Array.isArray(responder)) {
      const next = queue ? queue.shift() : undefined;
      if (next === undefined) {
        throw new Error('mock fetch: no queued response remaining');
      }
      spec = next;
    } else {
      spec = responder;
    }
    return toResponse(spec);
  };

  return {
    fetch: fetchImpl,
    calls,
    lastCall(): RecordedRequest {
      const call = calls.at(-1);
      if (call === undefined) {
        throw new Error('mock fetch: no calls recorded');
      }
      return call;
    },
  };
}

function urlOf(input: FetchInput): string {
  if (typeof input === 'string') {
    return input;
  }
  if (input instanceof URL) {
    return input.toString();
  }
  return input.url;
}

function normalizeHeaders(headers: unknown): Record<string, string> {
  const out: Record<string, string> = {};
  if (!headers) {
    return out;
  }
  if (headers instanceof Headers) {
    headers.forEach((value, key) => {
      out[key] = value;
    });
    return out;
  }
  if (Array.isArray(headers)) {
    for (const entry of headers) {
      if (Array.isArray(entry) && typeof entry[0] === 'string' && typeof entry[1] === 'string') {
        out[entry[0]] = entry[1];
      }
    }
    return out;
  }
  for (const [key, value] of Object.entries(headers as Record<string, unknown>)) {
    if (typeof value === 'string') {
      out[key] = value;
    }
  }
  return out;
}

function parseBody(body: FetchInit['body']): unknown {
  if (body === null || body === undefined) {
    return undefined;
  }
  if (typeof body === 'string') {
    try {
      return JSON.parse(body) as unknown;
    } catch {
      return body;
    }
  }
  return body;
}

function toResponse(spec: MockResponseSpec): Response {
  const status = spec.status ?? 200;
  const headers = new Headers(spec.headers);

  let body: string | null;
  if (spec.rawBody !== undefined) {
    body = spec.rawBody;
  } else if (spec.body === undefined) {
    body = null;
  } else if (typeof spec.body === 'string') {
    body = spec.body;
  } else {
    body = JSON.stringify(spec.body);
    if (!headers.has('content-type')) {
      headers.set('content-type', 'application/json');
    }
  }
  return new Response(body, { status, headers });
}
