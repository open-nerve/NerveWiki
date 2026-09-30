import { describe, expect, test } from "vitest";

import { darkScheme, memoryStorage } from "../test/fakes";
import { PreferencesStore, themeKey, type PreferenceSources } from "./preferences.store";

function store(sources: Partial<PreferenceSources> = {}) {
  return new PreferencesStore({
    storage: memoryStorage(),
    darkScheme: darkScheme(false),
    languages: ["en-US"],
    ...sources,
  });
}

describe("PreferencesStore theme", () => {
  test("follows the system until a theme is chosen", () => {
    const system = darkScheme(true);
    const prefs = store({ darkScheme: system });

    expect([prefs.theme, prefs.resolvedTheme]).toEqual(["system", "dark"]);
    system.change(false);
    expect(prefs.resolvedTheme).toBe("light");
  });

  test("keeps the chosen theme, and stores it for the next visit", () => {
    const storage = memoryStorage();
    const prefs = store({ storage, darkScheme: darkScheme(true) });

    prefs.setTheme("light");

    expect(prefs.resolvedTheme).toBe("light");
    expect(storage.values[themeKey]).toBe("light");
    expect(store({ storage }).theme).toBe("light");
  });

  test("ignores a stored value that is not a theme", () => {
    expect(store({ storage: memoryStorage({ [themeKey]: "sepia" }) }).theme).toBe("system");
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
    const prefs = store({ storage: blocked });

    prefs.setTheme("dark");
    prefs.setLocale("zh-CN");

    expect([prefs.resolvedTheme, prefs.locale]).toEqual(["dark", "zh-CN"]);
  });
});

describe("PreferencesStore locale", () => {
  test.each([
    [["zh-TW", "en"], "zh-CN"],
    [["fr-FR", "en-GB", "zh-CN"], "en"],
    [["de", "ZH"], "zh-CN"],
    [["fr"], "en"],
    [[], "en"],
  ])("follows the browser's languages %j: %s", (languages, want) => {
    expect(store({ languages }).locale).toBe(want);
  });

  test("keeps the chosen language, and stores it for the next visit", () => {
    const storage = memoryStorage();
    const prefs = store({ storage, languages: ["en"] });

    prefs.setLocale("zh-CN");

    expect(store({ storage, languages: ["en"] }).locale).toBe("zh-CN");
  });
});
