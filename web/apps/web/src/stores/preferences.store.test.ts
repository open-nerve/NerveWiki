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

describe("PreferencesStore last workspace", () => {
  test("is stored for the device: another tab, or the next visit, reads it", () => {
    const storage = memoryStorage();
    const prefs = store({ storage });
    const otherTab = store({ storage });
    expect(prefs.lastWorkspace()).toBeUndefined();

    prefs.setLastWorkspace("acme");
    expect([otherTab.lastWorkspace(), storage.values["nwiki.workspace"]]).toEqual(["acme", "acme"]);

    otherTab.setLastWorkspace("beta");
    expect(prefs.lastWorkspace()).toBe("beta");
  });

  test("lasts for this page when storage throws", () => {
    const blocked = {
      getItem: () => {
        throw new DOMException("blocked", "SecurityError");
      },
      setItem: () => {
        throw new DOMException("blocked", "SecurityError");
      },
    };
    const prefs = store({ storage: blocked });

    prefs.setLastWorkspace("acme");

    expect(prefs.lastWorkspace()).toBe("acme");
  });
});

test("PreferencesStore last workspace: one the storage failed to keep comes first", () => {
  const values: Record<string, string> = { "nwiki.workspace": "old" };
  const full = {
    getItem: (key: string) => values[key] ?? null,
    setItem: () => {
      throw new DOMException("full", "QuotaExceededError");
    },
  };
  const prefs = store({ storage: full });
  expect(prefs.lastWorkspace()).toBe("old");

  prefs.setLastWorkspace("acme");

  expect(prefs.lastWorkspace()).toBe("acme");
});

// The page's right column folded or not, this browser's (M13/P4/S1, decision 859).
describe("PreferencesStore right column", () => {
  test("is open until it is folded away, which is stored for the next visit", () => {
    const storage = memoryStorage();
    const prefs = store({ storage });
    expect(prefs.panelOpen).toBe(true);

    prefs.setPanelOpen(false);
    expect([prefs.panelOpen, storage.values["nwiki.panel"]]).toEqual([false, "closed"]);
    expect(store({ storage }).panelOpen).toBe(false);

    prefs.setPanelOpen(true);
    expect([prefs.panelOpen, storage.values["nwiki.panel"]]).toEqual([true, "open"]);
  });

  test("stored folded, it is folded; anything else stored, open", () => {
    expect(store({ storage: memoryStorage({ "nwiki.panel": "closed" }) }).panelOpen).toBe(false);
    expect(store({ storage: memoryStorage({ "nwiki.panel": "open" }) }).panelOpen).toBe(true);
    expect(store({ storage: memoryStorage({ "nwiki.panel": "shut" }) }).panelOpen).toBe(true);
  });

  test("folds for this page when storage throws", () => {
    const blocked = {
      getItem: () => {
        throw new DOMException("blocked", "SecurityError");
      },
      setItem: () => {
        throw new DOMException("blocked", "SecurityError");
      },
    };
    const prefs = store({ storage: blocked });
    expect(prefs.panelOpen).toBe(true);

    prefs.setPanelOpen(false);
    expect(prefs.panelOpen).toBe(false);
  });
});
