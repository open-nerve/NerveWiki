import { observer } from "mobx-react-lite";
import { createContext, use, useEffect, useMemo, type ReactNode } from "react";

import { useStore } from "../stores/context";
import type { Locale } from "./locale";
import { en, type MessageKey, type Messages } from "./messages/en";
import { zhCN } from "./messages/zh-CN";

const catalogs: Record<Locale, Messages> = { en, "zh-CN": zhCN };

/** Translate returns the text of key, with each {name} replaced by params.name. */
export type Translate = (key: MessageKey, params?: Record<string, string | number>) => string;

export function translator(locale: Locale): Translate {
  const messages = catalogs[locale];
  return (key, params) =>
    messages[key].replace(/\{(\w+)\}/g, (placeholder, name: string) =>
      params?.[name] === undefined ? placeholder : String(params[name])
    );
}

const I18nContext = createContext<Translate | null>(null);

/**
 * I18nProvider gives the text of the chosen language to useT, and keeps
 * <html lang> in step. Every component that calls useT renders again when the
 * language changes, observer or not.
 */
export const I18nProvider = observer(function I18nProvider({ children }: { children: ReactNode }) {
  const locale = useStore().preferences.locale;
  const t = useMemo(() => translator(locale), [locale]);
  useEffect(() => {
    document.documentElement.lang = locale;
  }, [locale]);
  return <I18nContext value={t}>{children}</I18nContext>;
});

export function useT(): Translate {
  const t = use(I18nContext);
  if (!t) {
    throw new Error("useT is used outside an I18nProvider");
  }
  return t;
}
