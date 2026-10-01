import { createClient, type ApiClient, type AuthTokens } from "@nervewiki/api-client";

import type { User } from "../services/account.service";
import type { InstanceInfo } from "../services/instance.service";
import type { Notebook } from "../services/notebook.service";
import type { Workspace } from "../services/workspace.service";
import { Session, type SessionDeps } from "../session/session";
import { AUTH_KEY } from "../session/token-manager";
import { PreferencesStore, type DarkSchemeQuery } from "../stores/preferences.store";
import { AppStores } from "../stores/root.store";

/** memoryStorage is a Storage of this page only, holding values. */
export function memoryStorage(
  values: Record<string, string> = {}
): SessionDeps["storage"] & { values: Record<string, string> } {
  return {
    values,
    getItem: (key) => values[key] ?? null,
    setItem: (key, value) => {
      values[key] = value;
    },
    removeItem: (key) => {
      delete values[key];
    },
  };
}

/** darkScheme is a prefers-color-scheme query whose answer the test changes. */
export function darkScheme(matches: boolean): DarkSchemeQuery & { change(dark: boolean): void } {
  const listeners: ((event: { matches: boolean }) => void)[] = [];
  return {
    matches,
    addEventListener: (_type, listener) => void listeners.push(listener),
    change: (dark) => {
      for (const listener of listeners) {
        listener({ matches: dark });
      }
    },
  };
}

/** preferences are the preferences of an English browser in light mode, stored nowhere. */
function preferences(): PreferencesStore {
  return new PreferencesStore({ storage: memoryStorage(), darkScheme: darkScheme(false), languages: ["en-US"] });
}

/** instanceJSON is a valid answer to GET /api/v0/instance. */
export const instanceJSON: InstanceInfo = {
  product: "Nerve Wiki",
  version: "1.2.3",
  commit: "4f2a9c1",
  api_version: "v0",
  signup_enabled: true,
  workspace_creation_enabled: true,
};

/** tokensJSON is a valid answer to a sign-in, a sign-up or a refresh. */
export const tokensJSON: AuthTokens = {
  token_type: "Bearer",
  access_token: "at-1",
  access_token_expires_in: 900,
  refresh_token: "rt-1",
  refresh_token_expires_at: "2026-10-31T00:00:00Z",
};

/** userJSON is a valid answer to GET /api/v0/me: an account done with onboarding. */
export const userJSON: User = {
  id: "0199a2b4-0000-7000-8000-000000000001",
  email: "ada@example.com",
  display_name: "Ada",
  onboarding_steps: ["profile", "workspace", "notebook"],
};

/** workspaceJSON is a workspace of userJSON's, which it administers. */
export const workspaceJSON: Workspace = {
  id: "0199a2b4-0000-7000-8000-0000000000b1",
  slug: "lab",
  name: "Lab",
  role: "admin",
  created_at: "2026-10-01T08:00:00Z",
  updated_at: "2026-10-01T08:00:00Z",
};

/** notebookJSON is a notebook of workspaceJSON's, userJSON's own: private, with userJSON its admin. */
export const notebookJSON: Notebook = {
  id: "0199a2b4-0000-7000-8000-0000000000c1",
  workspace_id: workspaceJSON.id,
  name: "Plans",
  workspace_access: "none",
  role: "admin",
  member_count: 1,
  created_at: "2026-10-02T08:00:00Z",
  updated_at: "2026-10-02T08:00:00Z",
};

/** Answer answers a request of the fake API. */
export type Answer = (request: Request) => Response | Promise<Response>;

/** fakeApi is an API client whose every request answer answers. */
export function fakeApi(answer: Answer): ApiClient {
  return createClient({ baseUrl: "http://nervewiki.test", fetch: async (request: Request) => answer(request) });
}

/** The record of a stored session, as a tab signed in to loginId keeps it. */
export function storedSession(loginId: string, refreshToken = "rt-0"): Record<string, string> {
  return { [AUTH_KEY]: JSON.stringify({ refresh_token: refreshToken, login_id: loginId }) };
}

/**
 * testSession is a session of one tab against the fake API answer: signed
 * out unless stored holds a session's record; stored is the tab's storage,
 * which the test can read. Its locks run at once, and each login gets the
 * next login id, login-1, login-2 …
 */
function testSession(answer: Answer, stored: Record<string, string> = {}): Session {
  let logins = 0;
  return new Session({
    storage: memoryStorage(stored),
    onStorage: () => () => {},
    locks: { request: (_name: string, task: () => Promise<unknown>) => task() } as unknown as SessionDeps["locks"],
    now: () => Date.now(),
    randomHex: () => `login-${++logins}`,
    client: { baseUrl: "http://nervewiki.test", fetch: async (request: Request) => answer(request) },
  });
}

/**
 * byRoute answers each request by its "METHOD /path" in routes, else by
 * the first key whose * each stand for a segment of the path, such as
 * "DELETE /api/v0/notebooks/*"; any other request is not found.
 */
export function byRoute(routes: Record<string, Answer>): Answer {
  return (request) => {
    const route = `${request.method} ${new URL(request.url).pathname}`;
    const answer = routes[route] ?? Object.entries(routes).find(([key]) => pattern(key)?.test(route))?.[1];
    return answer === undefined ? problem(404, "not_found") : answer(request);
  };
}

/** pattern is what a key with * matches, each * a segment of the path; none for a key without. */
function pattern(key: string): RegExp | undefined {
  if (!key.includes("*")) {
    return undefined;
  }
  const parts = key.split("*").map((part) => part.replaceAll(/[.+?^${}()|[\]\\]/g, String.raw`\$&`));
  return new RegExp(`^${parts.join("[^/]+")}$`);
}

/**
 * signedInApp is the page's stores of a tab signed in (from its stored
 * session, login-0) as userJSON, a member of workspaceJSON alone, which
 * sees no notebook; routes adds to or replaces the answers to the refresh,
 * GET /me, GET /instance, GET /workspaces and every workspace's GET
 * notebooks.
 */
export function signedInApp(routes: Record<string, Answer> = {}): AppStores {
  return testApp(
    byRoute({
      "POST /api/v0/auth/refresh": () => json(tokensJSON),
      "GET /api/v0/me": () => json(userJSON),
      "GET /api/v0/instance": () => json(instanceJSON),
      "GET /api/v0/workspaces": () => json({ data: [workspaceJSON] }),
      "GET /api/v0/workspaces/*/notebooks": () => json({ data: [] }),
      ...routes,
    }),
    storedSession("login-0")
  );
}

/** testApp is the page's stores over testSession(answer, stored). */
export function testApp(answer: Answer = () => json(instanceJSON), stored?: Record<string, string>): AppStores {
  return new AppStores(preferences(), testSession(answer, stored));
}

export function json(body: unknown, status = 200, contentType = "application/json"): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": contentType } });
}

/** problem is an application/problem+json answer with code, and more members if given. */
export function problem(
  status: number,
  code: string,
  more: Record<string, unknown> = {},
  headers: HeadersInit = {}
): Response {
  return new Response(JSON.stringify({ status, code, title: code, ...more }), {
    status,
    headers: { "Content-Type": "application/problem+json", ...headers },
  });
}
