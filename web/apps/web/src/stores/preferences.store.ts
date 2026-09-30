import { makeAutoObservable } from "mobx";

export type ThemePreference = "system" | "light" | "dark";
export type Theme = "light" | "dark";

/**
 * themeKey is where the theme preference is stored. public/theme-init.js
 * reads the same key, before the first paint; a test holds the two together.
 */
export const themeKey = "nervewiki.theme";

/** The part of localStorage the preferences use. */
export type PreferenceStorage = Pick<Storage, "getItem" | "setItem">;

/** The part of matchMedia("(prefers-color-scheme: dark)") the theme follows. */
export interface DarkSchemeQuery {
  readonly matches: boolean;
  addEventListener(type: "change", listener: (event: { matches: boolean }) => void): void;
}

/**
 * PreferencesStore holds this browser's display preferences. They belong to
 * the device, not to a login: the store outlives every RootStore.
 */
export class PreferencesStore {
  theme: ThemePreference;
  private systemDark: boolean;

  constructor(
    private readonly storage: PreferenceStorage,
    darkScheme: DarkSchemeQuery
  ) {
    this.theme = readTheme(storage);
    this.systemDark = darkScheme.matches;
    darkScheme.addEventListener("change", (event) => this.setSystemDark(event.matches));
    makeAutoObservable<this, "storage">(this, { storage: false });
  }

  /** resolvedTheme is the theme to show: the preference, or the system's. */
  get resolvedTheme(): Theme {
    if (this.theme === "system") {
      return this.systemDark ? "dark" : "light";
    }
    return this.theme;
  }

  setTheme(theme: ThemePreference): void {
    this.theme = theme;
    write(this.storage, themeKey, theme);
  }

  private setSystemDark(dark: boolean): void {
    this.systemDark = dark;
  }
}

function readTheme(storage: PreferenceStorage): ThemePreference {
  const stored = read(storage, themeKey);
  return stored === "light" || stored === "dark" ? stored : "system";
}

// Storage throws where the browser forbids it (blocked site data, a full
// quota): a preference is then kept for this page only.
function read(storage: PreferenceStorage, key: string): string | null {
  try {
    return storage.getItem(key);
  } catch {
    return null;
  }
}

function write(storage: PreferenceStorage, key: string, value: string): void {
  try {
    storage.setItem(key, value);
  } catch {
    // see read
  }
}
