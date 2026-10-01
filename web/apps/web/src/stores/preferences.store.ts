import { makeAutoObservable } from "mobx";

import { isLocale, localeFor, type Locale } from "../i18n/locale";

/** The theme preferences, in the order the app offers them. */
export const themePreferences = ["system", "light", "dark"] as const;
export type ThemePreference = (typeof themePreferences)[number];

export function isThemePreference(value: unknown): value is ThemePreference {
  return themePreferences.some((theme) => theme === value);
}
export type Theme = "light" | "dark";

/**
 * themeKey is where the theme preference is stored. public/theme-init.js
 * reads the same key, before the first paint; a test holds the two together.
 */
export const themeKey = "nwiki.theme";
const localeKey = "nwiki.locale";
/** workspaceKey is where the slug of the workspace this device showed last is stored (M2/P5 design 3.3). */
const workspaceKey = "nwiki.workspace";

/** The part of localStorage the preferences use. */
export type PreferenceStorage = Pick<Storage, "getItem" | "setItem">;

/** The part of matchMedia("(prefers-color-scheme: dark)") the theme follows. */
export interface DarkSchemeQuery {
  readonly matches: boolean;
  addEventListener(type: "change", listener: (event: { matches: boolean }) => void): void;
}

/** What the preferences read from the browser. */
export interface PreferenceSources {
  storage: PreferenceStorage;
  darkScheme: DarkSchemeQuery;
  /** navigator.languages: the browser's preferred languages, most preferred first. */
  languages: readonly string[];
}

/**
 * PreferencesStore holds this browser's display preferences, the theme and
 * the language, and the workspace it showed last. They belong to the
 * device, not to a login: the store outlives every RootStore.
 */
export class PreferencesStore {
  theme: ThemePreference;
  locale: Locale;
  private systemDark: boolean;
  private readonly storage: PreferenceStorage;
  /** The last workspace of this page, for when the storage cannot keep it. */
  private workspace: string | undefined = undefined;

  constructor({ storage, darkScheme, languages }: PreferenceSources) {
    this.storage = storage;
    const theme = read(storage, themeKey);
    this.theme = isThemePreference(theme) ? theme : "system";
    const locale = read(storage, localeKey);
    this.locale = isLocale(locale) ? locale : localeFor(languages);
    this.systemDark = darkScheme.matches;
    darkScheme.addEventListener("change", (event) => this.setSystemDark(event.matches));
    makeAutoObservable<this, "storage" | "workspace">(this, { storage: false, workspace: false });
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

  /** setLocale chooses the language; until then it follows the browser's. */
  setLocale(locale: Locale): void {
    this.locale = locale;
    write(this.storage, localeKey, locale);
  }

  /**
   * lastWorkspace is the slug of the workspace this device showed last, in
   * any of its tabs: it is read from the storage each time. It is not
   * observed; the landing reads it once.
   */
  lastWorkspace(): string | undefined {
    return read(this.storage, workspaceKey) ?? this.workspace;
  }

  setLastWorkspace(slug: string): void {
    this.workspace = slug;
    write(this.storage, workspaceKey, slug);
  }

  private setSystemDark(dark: boolean): void {
    this.systemDark = dark;
  }
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
