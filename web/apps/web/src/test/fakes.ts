import { createClient, type ApiClient } from "@nervewiki/api-client";

import type { InstanceInfo } from "../services/instance.service";
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
 * out unless stored holds a session's record; its locks run at once, and
 * each login gets the next login id, login-1, login-2 …
 */
function testSession(answer: Answer, stored: Record<string, string> = {}): Session {
  let logins = 0;
  return new Session({
    storage: memoryStorage({ ...stored }),
    onStorage: () => () => {},
    locks: { request: (_name: string, task: () => Promise<unknown>) => task() } as unknown as SessionDeps["locks"],
    now: () => Date.now(),
    randomHex: () => `login-${++logins}`,
    client: { baseUrl: "http://nervewiki.test", fetch: async (request: Request) => answer(request) },
  });
}

/** testApp is the page's stores over testSession(answer, stored). */
export function testApp(answer: Answer = () => json(instanceJSON), stored?: Record<string, string>): AppStores {
  return new AppStores(preferences(), testSession(answer, stored));
}

export function json(body: unknown, status = 200, contentType = "application/json"): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": contentType } });
}
