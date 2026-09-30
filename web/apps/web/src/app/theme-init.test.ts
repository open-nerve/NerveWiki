import { afterEach, expect, test, vi } from "vitest";

import script from "../../public/theme-init.js?raw";
import { PreferencesStore, themeKey, type ThemePreference } from "../stores/preferences.store";
import { darkScheme, memoryStorage } from "../test/fakes";

// public/theme-init.js runs before the app, from the same stored preference:
// the two must agree on the key and on the theme they show.

afterEach(() => {
  localStorage.clear();
  document.documentElement.className = "";
});

test("theme-init.js reads the store's key", () => {
  expect(script).toContain(JSON.stringify(themeKey));
});

test.each<[ThemePreference | null, boolean]>([
  [null, false],
  [null, true],
  ["system", true],
  ["light", true],
  ["dark", false],
])("stored %s with a dark system %s: theme-init.js shows what the store resolves", (stored, systemDark) => {
  if (stored !== null) {
    localStorage.setItem(themeKey, stored);
  }
  vi.stubGlobal("matchMedia", () => ({ matches: systemDark }));

  new Function(script)();

  const store = new PreferencesStore(
    memoryStorage(stored === null ? {} : { [themeKey]: stored }),
    darkScheme(systemDark)
  );
  expect(document.documentElement.classList.contains("dark")).toBe(store.resolvedTheme === "dark");
});
