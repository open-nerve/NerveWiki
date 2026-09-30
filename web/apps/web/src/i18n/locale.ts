export type Locale = "zh-CN" | "en";

/** The languages of the app, each named in itself. */
export const locales: readonly { value: Locale; name: string }[] = [
  { value: "zh-CN", name: "简体中文" },
  { value: "en", name: "English" },
];

export function isLocale(value: unknown): value is Locale {
  return locales.some((locale) => locale.value === value);
}

/**
 * localeFor picks the app's language for the browser's preferred languages
 * (navigator.languages, most preferred first): the first that is Chinese or
 * English, English otherwise.
 */
export function localeFor(languages: readonly string[]): Locale {
  for (const language of languages) {
    const primary = language.toLowerCase().split("-")[0];
    if (primary === "zh") {
      return "zh-CN";
    }
    if (primary === "en") {
      return "en";
    }
  }
  return "en";
}
