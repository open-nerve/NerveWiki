import { createContext, use, useMemo, type ReactNode } from "react";

import type { Locale } from "./locale";
import { en, type MessageKey, type Messages } from "./messages/en";
import { zhCN } from "./messages/zh-CN";

const catalogs: Record<Locale, Messages> = { en, "zh-CN": zhCN };

/** The names of the {placeholders} in text. */
type Placeholders<Text extends string> = Text extends `${string}{${infer Name}}${infer Rest}`
  ? Name | Placeholders<Rest>
  : never;

/** The parameters of key: a value for each of its placeholders, or none if it has none. */
type Params<Key extends MessageKey> = [Placeholders<(typeof en)[Key]>] extends [never]
  ? []
  : [params: Record<Placeholders<(typeof en)[Key]>, string | number>];

/** Translate returns the text of key, with each {name} replaced by params.name. */
export type Translate = <Key extends MessageKey>(key: Key, ...params: Params<Key>) => string;

export function translator(locale: Locale): Translate {
  const messages = catalogs[locale];
  return (key, ...[params]) =>
    messages[key].replace(/\{(\w+)\}/g, (placeholder, name: string) => {
      const value = (params as Record<string, string | number> | undefined)?.[name];
      return value === undefined ? placeholder : String(value);
    });
}

const I18nContext = createContext<Translate | null>(null);

/**
 * I18nProvider gives the text of locale to useT: every component that calls
 * useT renders again when the locale changes.
 */
export function I18nProvider({ locale, children }: { locale: Locale; children: ReactNode }) {
  const t = useMemo(() => translator(locale), [locale]);
  return <I18nContext value={t}>{children}</I18nContext>;
}

export function useT(): Translate {
  const t = use(I18nContext);
  if (!t) {
    throw new Error("useT is used outside an I18nProvider");
  }
  return t;
}
