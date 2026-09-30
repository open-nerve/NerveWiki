import { createClient, type ApiClient } from "@nervewiki/api-client";

import type { InstanceInfo } from "../services/instance.service";
import { PreferencesStore, type DarkSchemeQuery, type PreferenceStorage } from "../stores/preferences.store";

/** memoryStorage is a PreferenceStorage holding values. */
export function memoryStorage(
  values: Record<string, string> = {}
): PreferenceStorage & { values: Record<string, string> } {
  return {
    values,
    getItem: (key) => values[key] ?? null,
    setItem: (key, value) => {
      values[key] = value;
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
export function preferences(): PreferencesStore {
  return new PreferencesStore({ storage: memoryStorage(), darkScheme: darkScheme(false), languages: ["en-US"] });
}

/** instanceJSON is a valid answer to GET /api/v0/instance. */
export const instanceJSON: InstanceInfo = {
  product: "Nerve Wiki",
  version: "1.2.3",
  commit: "4f2a9c1",
  api_version: "v0",
};

/**
 * fakeApi is an API client whose requests answer takes; it throws for a
 * request it does not expect.
 */
export function fakeApi(answer: (request: Request) => Response | Promise<Response>): ApiClient {
  return createClient({ baseUrl: "http://nervewiki.test", fetch: async (request: Request) => answer(request) });
}

export function json(body: unknown, status = 200, contentType = "application/json"): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": contentType } });
}
