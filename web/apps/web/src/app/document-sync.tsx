import { observer } from "mobx-react-lite";
import { useEffect } from "react";

import { useStore } from "../stores/context";

/**
 * DocumentSync keeps <html> in step with the preferences: lang with the
 * language, the dark class with the resolved theme. It sits with the
 * providers, above the router, so that it stays when an error page replaces
 * the layout.
 */
export const DocumentSync = observer(function DocumentSync() {
  const { locale, resolvedTheme } = useStore().preferences;
  useEffect(() => {
    document.documentElement.lang = locale;
  }, [locale]);
  useEffect(() => {
    document.documentElement.classList.toggle("dark", resolvedTheme === "dark");
  }, [resolvedTheme]);
  return null;
});
