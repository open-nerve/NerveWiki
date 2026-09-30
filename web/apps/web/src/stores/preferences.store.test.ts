import { describe, expect, test } from "vitest";

import { darkScheme, memoryStorage } from "../test/fakes";
import { PreferencesStore, themeKey } from "./preferences.store";

describe("PreferencesStore theme", () => {
  test("follows the system until a theme is chosen", () => {
    const system = darkScheme(true);
    const store = new PreferencesStore(memoryStorage(), system);

    expect([store.theme, store.resolvedTheme]).toEqual(["system", "dark"]);
    system.change(false);
    expect(store.resolvedTheme).toBe("light");
  });

  test("keeps the chosen theme, and stores it for the next visit", () => {
    const storage = memoryStorage();
    const store = new PreferencesStore(storage, darkScheme(true));

    store.setTheme("light");

    expect(store.resolvedTheme).toBe("light");
    expect(storage.values[themeKey]).toBe("light");
    expect(new PreferencesStore(storage, darkScheme(true)).theme).toBe("light");
  });

  test("ignores a stored value that is not a theme", () => {
    const store = new PreferencesStore(memoryStorage({ [themeKey]: "sepia" }), darkScheme(false));

    expect(store.theme).toBe("system");
  });

  test("works for this page when storage throws", () => {
    const blocked = {
      getItem: () => {
        throw new DOMException("blocked", "SecurityError");
      },
      setItem: () => {
        throw new DOMException("blocked", "SecurityError");
      },
    };
    const store = new PreferencesStore(blocked, darkScheme(false));

    store.setTheme("dark");

    expect([store.theme, store.resolvedTheme]).toEqual(["dark", "dark"]);
  });
});
