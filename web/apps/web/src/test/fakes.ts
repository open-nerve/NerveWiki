import type { DarkSchemeQuery, PreferenceStorage } from "../stores/preferences.store";

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
