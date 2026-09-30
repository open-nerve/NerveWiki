// A test double of the server's API behind fetch, for the auth tests: every request waits until the test
// answers it or fails it, so the tests set the order of events themselves.

import { createClient } from "@nervewiki/api-client";
import type { AuthTokens } from "@nervewiki/api-client";

/** A request the fake received and has not answered yet. */
export type Call = {
  method: string;
  path: string;
  /** The parameters of the query string. */
  query: Record<string, string>;
  authorization: string | null;
  body: unknown;
  /** When the request arrived, on the fake clock. */
  at: number;
  /** Whether the client gave the request up (its signal aborted). */
  aborted: () => boolean;
  answer: (response: Response) => void;
  /** Rejects the request as a browser's fetch does when the network fails: with a TypeError. */
  fail: () => void;
};

const BASE_URL = "http://nervewiki.test";

export class FakeServer {
  readonly calls: Call[] = [];
  private issued = 0;

  /** The fetch of the client under test: reads the body as fetch does, parks the request until answered or failed. */
  fetch = async (request: Request): Promise<Response> => {
    const text = await request.text();
    const url = new URL(request.url);
    return new Promise<Response>((resolve, reject) => {
      const onAbort = () => reject(new DOMException("The operation was aborted.", "AbortError"));
      if (request.signal.aborted) return onAbort();
      request.signal.addEventListener("abort", onAbort);
      this.calls.push({
        method: request.method,
        path: url.pathname,
        query: Object.fromEntries(url.searchParams),
        authorization: request.headers.get("Authorization"),
        body: text === "" ? undefined : (JSON.parse(text) as unknown),
        at: Date.now(),
        aborted: () => request.signal.aborted,
        answer: resolve,
        fail: () => reject(new TypeError("Failed to fetch")),
      });
    });
  };

  /** A client of this fake, as the app makes one. */
  client() {
    return createClient({ baseUrl: BASE_URL, fetch: this.fetch });
  }

  /** The requests to path so far. */
  to(path: string): Call[] {
    return this.calls.filter((c) => c.path === path);
  }

  /** The next pair of tokens, numbered: rt-1 with at-1, rt-2 with at-2 … */
  tokens(expiresIn = 900): AuthTokens {
    this.issued++;
    return {
      token_type: "Bearer",
      access_token: `at-${this.issued}`,
      access_token_expires_in: expiresIn,
      refresh_token: `rt-${this.issued}`,
      refresh_token_expires_at: "2026-10-27T00:00:00Z",
    };
  }
}

export function json(status: number, body: unknown, headers: Record<string, string> = {}): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": status < 400 ? "application/json" : "application/problem+json", ...headers },
  });
}

export function problem(status: number, code: string, headers: Record<string, string> = {}): Response {
  return json(status, { status, code, title: "", detail: code }, headers);
}

export const noContent = () => new Response(null, { status: 204 });
